package balancer

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"net"
	"sync"

	"github.com/weaveworks/flux/balancer/events"
	"github.com/weaveworks/flux/balancer/forwarder"
	"github.com/weaveworks/flux/balancer/model"
	"github.com/weaveworks/flux/common/daemon"
)

type servicesConfig struct {
	netConfig netConfig
	updates   <-chan model.ServiceUpdate
	*ipTables
	eventHandler events.Handler
	errorSink    daemon.ErrorSink
	done         chan<- model.ServiceUpdate
}

type services struct {
	servicesConfig

	lock     sync.Mutex
	stopped  chan struct{}
	finished chan struct{}
	services map[string]*service
}

func (cf servicesConfig) start() *services {
	svcs := &services{
		servicesConfig: cf,

		stopped:  make(chan struct{}),
		finished: make(chan struct{}),
		services: make(map[string]*service),
	}
	go svcs.run()
	return svcs
}

func (svcs *services) stop() {
	svcs.lock.Lock()
	defer svcs.lock.Unlock()

	if svcs.services != nil {
		close(svcs.stopped)
		<-svcs.finished

		for _, svc := range svcs.services {
			svc.close()
		}

		svcs.services = nil
	}
}

func (svcs *services) run() {
	for {
		select {
		case <-svcs.stopped:
			close(svcs.finished)
			return

		case update := <-svcs.updates:
			svcs.doUpdate(update)
			if svcs.done != nil {
				svcs.done <- update
			}
		}
	}
}

func (svcs *services) doUpdate(update model.ServiceUpdate) {
	for name, ms := range update.Updates {
		svc := svcs.services[name]
		if svc == nil {
			if ms == nil {
				continue
			}

			svc, err := svcs.newService(ms)
			if err != nil {
				log.WithError(err).Error("adding service ",
					name)
				continue
			}

			svcs.services[name] = svc
		} else if ms != nil {
			err := svc.updateState(ms)
			if err != nil {
				log.WithError(err).Error("updating service ",
					name)
				continue
			}
		} else {
			delete(svcs.services, name)
			svc.close()
		}
	}

	if update.Reset {
		// Delete any services not in the model
		for name, svc := range svcs.services {
			if update.Updates[name] == nil {
				delete(svcs.services, name)
				svc.close()
			}
		}
	}
}

type service struct {
	*services
	state serviceState
}

type serviceState interface {
	stop()
	// return true to keep the same state; false to calculate a new
	// state
	update(*model.Service) (bool, error)
}

func (svcs *services) newService(update *model.Service) (*service, error) {
	svc := &service{services: svcs}
	if err := svc.updateState(update); err != nil {
		return nil, err
	}

	return svc, nil
}

func (svc *service) updateState(update *model.Service) error {
	if svc.state != nil {
		ok, err := svc.state.update(update)
		if err != nil || ok {
			return err
		}
	}

	// start the new forwarder before stopping the old one, to
	// avoid a window where there is no rule for the service
	var start func(*model.Service) (serviceState, error)
	if len(update.Instances) == 0 {
		start = svc.startRejecting
	} else {
		start = svc.startForwarding
	}

	state, err := start(update)
	if err != nil {
		return err
	}

	if svc.state != nil {
		svc.state.stop()
	}

	svc.state = state
	return nil
}

func (svc *service) close() {
	svc.state.stop()
	svc.state = nil
}

// When a service should reject packets
type rejecting func()

func (svc *service) startRejecting(s *model.Service) (serviceState, error) {
	log.Info("rejecting service: ", s.Summary())
	rule := []interface{}{
		"-p", "tcp",
		"-d", s.Address.IP(),
		"--dport", s.Address.Port(),
		"-j", "REJECT",
	}

	err := svc.ipTables.addRule("filter", rule)
	if err != nil {
		return nil, err
	}

	return rejecting(func() {
		svc.ipTables.deleteRule("filter", rule)
	}), nil
}

func (rej rejecting) stop() {
	rej()
}

func (rej rejecting) update(s *model.Service) (bool, error) {
	return len(s.Instances) == 0, nil
}

// When a service should forward packets
type forwarding struct {
	svc       *service
	service   *model.Service
	forwarder *forwarder.Forwarder
	rule      []interface{}
}

func (svc *service) startForwarding(s *model.Service) (serviceState, error) {
	log.Info("forwarding service: ", s.Summary())

	ip, err := bridgeIP(svc.netConfig.bridge)
	if err != nil {
		return nil, err
	}

	fwd, err := forwarder.Config{
		ServiceName:  s.Name,
		Description:  s.Description(),
		BindIP:       ip,
		EventHandler: svc.eventHandler,
		ErrorSink:    svc.errorSink,
	}.New()
	if err != nil {
		return nil, err
	}

	fwd.SetProtocol(s.Protocol)
	fwd.SetInstances(s.Instances)

	rule := []interface{}{
		"-p", "tcp",
		"-d", s.Address.IP(),
		"--dport", s.Address.Port(),
		"-j", "DNAT",
		"--to-destination", fwd.Addr(),
	}
	err = svc.ipTables.addRule("nat", rule)
	if err != nil {
		fwd.Stop()
		return nil, err
	}

	return forwarding{svc: svc, service: s, forwarder: fwd, rule: rule}, nil
}

func (fwd forwarding) stop() {
	fwd.forwarder.Stop()
	fwd.svc.ipTables.deleteRule("nat", fwd.rule)
}

func (fwd forwarding) update(s *model.Service) (bool, error) {
	if len(s.Instances) == 0 {
		// Need to switch to rejecting
		return false, nil
	}

	if s.Equal(fwd.service) {
		// Same address and same set of instances; stay as it is
		return true, nil
	}

	if s.Address == nil || !s.Address.Equal(*fwd.service.Address) {
		// Address changed, recreate
		return false, nil
	}

	log.Info("forwarding service: ", s.Summary())
	fwd.forwarder.SetProtocol(s.Protocol)
	fwd.forwarder.SetInstances(s.Instances)
	return true, nil
}

func bridgeIP(br string) (net.IP, error) {
	iface, err := net.InterfaceByName(br)
	if err != nil {
		return nil, err
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return nil, err
	}

	for _, addr := range addrs {
		if cidr, ok := addr.(*net.IPNet); ok {
			if ip := cidr.IP.To4(); ip != nil {
				return ip, nil
			}
		}
	}

	return nil, fmt.Errorf("no IPv4 address found on netdev %s", br)
}

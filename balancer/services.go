package balancer

import (
	log "github.com/Sirupsen/logrus"
	"sync"

	"github.com/weaveworks/flux/balancer/events"
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
	closed   chan struct{}
	finished chan struct{}
	services map[string]*service
}

func (cf servicesConfig) start() *services {
	svcs := &services{
		servicesConfig: cf,

		closed:   make(chan struct{}),
		finished: make(chan struct{}),
		services: make(map[string]*service),
	}
	go svcs.run()
	return svcs
}

func (svcs *services) close() {
	svcs.lock.Lock()
	defer svcs.lock.Unlock()

	if svcs.services != nil {
		close(svcs.closed)
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
		case <-svcs.closed:
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
	if !shouldForward(update) {
		start = notForwarding
	} else if len(update.Instances) == 0 {
		start = svc.startRejecting
	} else {
		start = forwardingConfig{
			netConfig:    svc.netConfig,
			ipTables:     svc.ipTables,
			eventHandler: svc.eventHandler,
			errorSink:    svc.errorSink,
		}.start
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

// If there's no address, don't forward. We will want more
// sophisticated rules later, if e.g., there are different kinds of
// forwarding.
func shouldForward(s *model.Service) bool {
	return s.IP != nil && s.Port > 0
}

// When a service shouldn't be forwarded
type notforwarding struct{}

func notForwarding(s *model.Service) (serviceState, error) {
	log.Debugf("moving service %s to state 'notForwarding'", s.Name)
	return notforwarding(struct{}{}), nil
}

func (_ notforwarding) stop() {
}

func (_ notforwarding) update(s *model.Service) (bool, error) {
	return !shouldForward(s), nil
}

// When a service should reject packets
type rejecting func()

func (svc *service) startRejecting(s *model.Service) (serviceState, error) {
	log.Info("rejecting service: ", s.Summary())
	rule := []interface{}{
		"-p", "tcp",
		"-d", s.IP,
		"--dport", s.Port,
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
	return shouldForward(s) && len(s.Instances) == 0, nil
}

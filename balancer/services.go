package balancer

import (
	log "github.com/Sirupsen/logrus"
	"sync"

	"github.com/squaremo/ambergreen/balancer/events"
	"github.com/squaremo/ambergreen/balancer/model"
	"github.com/squaremo/ambergreen/common/errorsink"
)

type servicesConfig struct {
	netConfig netConfig
	updates   <-chan model.ServiceUpdate
	*ipTables
	eventHandler events.Handler
	errorSink    errorsink.ErrorSink
	done         chan<- struct{}
}

type services struct {
	servicesConfig

	lock     sync.Mutex
	closed   chan struct{}
	finished chan struct{}
	services map[string]*service
}

func (cf servicesConfig) new() *services {
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
				svcs.done <- struct{}{}
			}
		}
	}
}

func (svcs *services) doUpdate(update model.ServiceUpdate) {
	svc := svcs.services[update.Name]
	if svc == nil {
		if update.Delete {
			return
		}

		svc, err := svcs.newService(&update.Service)
		if err != nil {
			log.Error("adding service ", update.Name, ": ",
				err)
			return
		}

		svcs.services[update.Name] = svc
	} else if !update.Delete {
		err := svc.update(&update.Service)
		if err != nil {
			log.Error("updating service ", update.Name, ": ",
				err)
			return
		}
	} else {
		delete(svcs.services, update.Name)
		svc.close()
	}
}

type service struct {
	*services
	state serviceState
}

type serviceState interface {
	stop()
	update(*model.Service) (bool, error)
}

func (svcs *services) newService(update *model.Service) (*service, error) {
	svc := &service{services: svcs}
	if err := svc.update(update); err != nil {
		return nil, err
	}

	return svc, nil
}

func (svc *service) update(update *model.Service) error {
	if svc.state != nil {
		ok, err := svc.state.update(update)
		if err != nil || ok {
			return err
		}
	}

	// start the new forwarder before stopping the old one, to
	// avoid a window where there is no rule for the service
	start := forwardingConfig{
		netConfig:    svc.netConfig,
		ipTables:     svc.ipTables,
		eventHandler: svc.eventHandler,
		errorSink:    svc.errorSink,
	}.start
	if len(update.Instances) == 0 {
		start = svc.startRejecting
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

type rejecting func()

func (svc *service) startRejecting(s *model.Service) (serviceState, error) {
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
	return len(s.Instances) == 0, nil
}

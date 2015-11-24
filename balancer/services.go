package balancer

import (
	log "github.com/Sirupsen/logrus"
	"sync"

	"github.com/squaremo/ambergreen/balancer/events"
	"github.com/squaremo/ambergreen/balancer/fatal"
	"github.com/squaremo/ambergreen/balancer/model"
)

type servicesConfig struct {
	netConfig netConfig
	updates   <-chan model.ServiceUpdate
	*ipTables
	eventHandler events.Handler
	fatalSink    fatal.Sink
	done         chan<- struct{}
}

type services struct {
	servicesConfig

	lock     sync.Mutex
	closed   chan struct{}
	finished chan struct{}
	services map[model.ServiceKey]*service
}

func (cf servicesConfig) new() *services {
	svcs := &services{
		servicesConfig: cf,

		closed:   make(chan struct{}),
		finished: make(chan struct{}),
		services: make(map[model.ServiceKey]*service),
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
	svc := svcs.services[update.ServiceKey]
	if svc == nil {
		if update.ServiceInfo == nil {
			return
		}

		svc, err := svcs.newService(update)
		if err != nil {
			log.Error("adding service ", update.ServiceKey, ": ",
				err)
			return
		}

		svcs.services[update.ServiceKey] = svc
	} else if update.ServiceInfo != nil {
		err := svc.update(update)
		if err != nil {
			log.Error("updating service ", update.ServiceKey, ": ",
				err)
			return
		}
	} else {
		delete(svcs.services, update.ServiceKey)
		svc.close()
	}
}

type service struct {
	*services
	key   model.ServiceKey
	state serviceState
}

type serviceState interface {
	stop()
	update(model.ServiceUpdate) (bool, error)
}

func (svcs *services) newService(upd model.ServiceUpdate) (*service, error) {
	svc := &service{
		services: svcs,
		key:      upd.ServiceKey,
	}

	err := svc.update(upd)
	if err != nil {
		return nil, err
	}

	return svc, nil
}

func (svc *service) update(upd model.ServiceUpdate) error {
	if svc.state != nil {
		ok, err := svc.state.update(upd)
		if err != nil || ok {
			return err
		}
	}

	// start the new forwarder before stopping the old one, to
	// avoid a window where there is no rule for the service
	start := svc.startForwarding
	if len(upd.Instances) == 0 {
		start = svc.startRejecting
	}

	state, err := start(upd)
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

func (svc *service) startRejecting(upd model.ServiceUpdate) (serviceState, error) {
	rule := []interface{}{
		"-p", "tcp",
		"-d", upd.IP(),
		"--dport", upd.Port,
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

func (rej rejecting) update(upd model.ServiceUpdate) (bool, error) {
	return len(upd.Instances) == 0, nil
}

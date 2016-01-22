package etcdcontrol

import (
	"net"

	log "github.com/Sirupsen/logrus"

	"github.com/squaremo/flux/common/daemon"
	"github.com/squaremo/flux/common/data"
	"github.com/squaremo/flux/common/store"

	"github.com/squaremo/flux/balancer/model"
)

type Listener struct {
	store   store.Store
	updates chan model.ServiceUpdate
}

func (l *Listener) send(serviceName string) {
	service, err := l.store.GetService(serviceName, store.QueryServiceOptions{WithInstances: true})
	if err != nil {
		log.Error(err)
		return
	}

	// It is OK to have an empty Address, but not to have a malformed
	// address
	ip := net.ParseIP(service.Address)
	if service.Address != "" && ip == nil {
		log.Errorf("Bad address \"%s\" for service %s",
			service.Address, serviceName)
		return
	}

	insts := []model.Instance{}
	for _, instance := range service.Instances {
		switch instance.State {
		case data.LIVE:
			break // i.e., proceed
		default:
			log.Debugf("Ignoring instance '%s', not marked as live", instance.Name)
			continue // try next instance
		}
		ip := net.ParseIP(instance.Address)
		if ip == nil {
			log.Errorf("Bad address \"%s\" for instance %s/%s",
				instance.Address, serviceName, service.Name)
			return
		}

		insts = append(insts, model.Instance{
			Name:  instance.Name,
			Group: instance.ContainerRule,
			IP:    ip,
			Port:  instance.Port,
		})
		log.Debugf("Added instance %s with address %s:%d to service %s", instance.Name, instance.Address, instance.Port, service.Name)
	}

	l.updates <- model.ServiceUpdate{
		Service: model.Service{
			Name:      serviceName,
			Protocol:  service.Protocol,
			IP:        ip,
			Port:      service.Port,
			Instances: insts,
		},
	}
}

func NewListener(store store.Store, errorSink daemon.ErrorSink) (*Listener, error) {
	listener := &Listener{
		store:   store,
		updates: make(chan model.ServiceUpdate),
	}
	go listener.run(errorSink)
	return listener, nil
}

func (l *Listener) Updates() <-chan model.ServiceUpdate {
	return l.updates
}

func (l *Listener) run(errorSink daemon.ErrorSink) {
	log.Debugf("Initialising state")
	changes := make(chan data.ServiceChange)
	l.store.WatchServices(nil, changes, errorSink,
		store.QueryServiceOptions{WithInstances: true})

	// Send initial state of each service
	store.ForeachServiceInstance(l.store, func(name string, _ data.Service) error {
		log.Debugf("Initialising state for service %s", name)
		l.send(name)
		return nil
	}, nil)

	for {
		change := <-changes
		if change.ServiceDeleted {
			l.updates <- model.ServiceUpdate{
				Service: model.Service{Name: change.Name},
				Delete:  true,
			}
		} else {
			log.Debugf("Updating state for service %s", change.Name)
			l.send(change.Name)
		}
	}
}

func (l *Listener) Close() {
	// TODO
}

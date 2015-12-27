package etcdcontrol

import (
	"net"

	log "github.com/Sirupsen/logrus"

	"github.com/squaremo/ambergreen/common/daemon"
	"github.com/squaremo/ambergreen/common/data"
	"github.com/squaremo/ambergreen/common/store"

	"github.com/squaremo/ambergreen/balancer/model"
)

type Listener struct {
	store   store.Store
	updates chan model.ServiceUpdate
}

func (l *Listener) send(serviceName string) {
	service, err := l.store.GetServiceDetails(serviceName)
	if err != nil {
		log.Error(err)
		return
	}

	ip := net.ParseIP(service.Address)
	if ip == nil {
		log.Errorf("Bad address \"%s\" for service %s",
			service.Address, serviceName)
		return
	}

	var insts []model.Instance
	l.store.ForeachInstance(serviceName, func(name string, instance data.Instance) {
		ip := net.ParseIP(instance.Address)
		if ip == nil {
			log.Errorf("Bad address \"%s\" for instance %s/%s",
				instance.Address, serviceName, name)
			return
		}

		insts = append(insts, model.Instance{
			Name:  name,
			Group: instance.ContainerGroup,
			IP:    ip,
			Port:  instance.Port,
		})
	})

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
	changes := make(chan data.ServiceChange)
	l.store.WatchServices(changes, nil, errorSink,
		store.WatchServicesOptions{WithInstanceChanges: true})

	// Send initial state of each service
	l.store.ForeachServiceInstance(func(name string, _ data.Service) {
		l.send(name)
	}, nil)

	for {
		change := <-changes
		if change.ServiceDeleted {
			l.updates <- model.ServiceUpdate{
				Service: model.Service{Name: change.Name},
				Delete:  true,
			}
		} else {
			l.send(change.Name)
		}
	}
}

func (l *Listener) Close() {
	// TODO
}

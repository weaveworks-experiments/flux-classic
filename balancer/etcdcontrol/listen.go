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
		ip := net.ParseIP(instance.Address)
		if ip == nil {
			log.Errorf("Bad address \"%s\" for instance %s/%s",
				instance.Address, serviceName, service.Name)
			return
		}

		insts = append(insts, model.Instance{
			Name:  instance.Name,
			Group: instance.ContainerGroup,
			IP:    ip,
			Port:  instance.Port,
		})
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
	changes := make(chan data.ServiceChange)
	l.store.WatchServices(changes, nil, errorSink,
		store.WatchServicesOptions{WithInstanceChanges: true})

	// Send initial state of each service
	store.ForeachServiceInstance(l.store, func(name string, _ data.Service) error {
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
			l.send(change.Name)
		}
	}
}

func (l *Listener) Close() {
	// TODO
}

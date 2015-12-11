package etcdcontrol

import (
	"log"
	"net"

	"github.com/squaremo/ambergreen/common/data"
	"github.com/squaremo/ambergreen/common/errorsink"
	"github.com/squaremo/ambergreen/common/store"
	"github.com/squaremo/ambergreen/common/store/etcdstore"

	"github.com/squaremo/ambergreen/balancer/model"
)

type Listener struct {
	store   store.Store
	updates chan model.ServiceUpdate
}

func (l *Listener) send(serviceName string) error {
	service, err := l.store.GetServiceDetails(serviceName)
	if err != nil {
		return err
	}
	update := model.ServiceUpdate{
		ServiceKey:  model.MakeServiceKey("tcp", net.ParseIP(service.Address), service.Port),
		ServiceInfo: &model.ServiceInfo{Protocol: service.Protocol},
	}
	l.store.ForeachInstance(serviceName, func(name string, instance data.Instance) {
		update.ServiceInfo.Instances = append(update.ServiceInfo.Instances, model.MakeInstance(name, instance.InstanceGroup, net.ParseIP(instance.Address), instance.Port))
	})
	log.Printf("Sending update for %s: %+v\n", update.ServiceKey.String(), update.ServiceInfo)
	l.updates <- update
	return nil
}

func NewListener(errorSink errorsink.ErrorSink) (*Listener, error) {
	listener := &Listener{
		store:   etcdstore.NewFromEnv(),
		updates: make(chan model.ServiceUpdate),
	}
	go listener.run(errorSink)
	return listener, nil
}

func (l *Listener) Updates() <-chan model.ServiceUpdate {
	return l.updates
}

func (l *Listener) run(errorSink errorsink.ErrorSink) {
	changes := make(chan data.ServiceChange)
	l.store.WatchServices(changes, nil, errorSink, true)

	// Send initial state of each service
	l.store.ForeachServiceInstance(func(name string, _ data.Service) {
		l.send(name)
	}, nil)

	for {
		change := <-changes
		l.send(change.Name)
	}
}

func (l *Listener) Close() {
	// TODO
}

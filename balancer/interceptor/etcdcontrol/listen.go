// Integrate coatl with ambergris via channel
package etcdcontrol

import (
	"log"
	"net"

	"github.com/squaremo/ambergreen/pkg/backends"
	"github.com/squaremo/ambergreen/pkg/data"

	"github.com/squaremo/ambergreen/balancer/interceptor/model"
)

type Listener struct {
	backend *backends.Backend
	updates chan model.ServiceUpdate
	errors  chan<- error
}

func (l *Listener) send(serviceName string) error {
	service, err := l.backend.GetServiceDetails(serviceName)
	if err != nil {
		return err
	}
	update := model.ServiceUpdate{
		ServiceKey:  model.MakeServiceKey("tcp", net.ParseIP(service.Address), service.Port),
		ServiceInfo: &model.ServiceInfo{Protocol: service.Protocol},
	}
	l.backend.ForeachInstance(serviceName, func(name string, instance data.Instance) {
		update.ServiceInfo.Instances = append(update.ServiceInfo.Instances, model.MakeInstance(net.ParseIP(instance.Address), instance.Port))
	})
	log.Printf("Sending update for %s: %+v\n", update.ServiceKey.String(), update.ServiceInfo)
	l.updates <- update
	return nil
}

func NewListener(errors chan<- error) (*Listener, error) {
	listener := &Listener{
		backend: backends.NewBackend([]string{}),
		updates: make(chan model.ServiceUpdate),
		errors:  errors,
	}
	go listener.run()
	return listener, nil
}

func (l *Listener) Updates() <-chan model.ServiceUpdate {
	return l.updates
}

func (l *Listener) run() {
	ch := l.backend.Watch()

	// Send initial state of each service
	l.backend.ForeachServiceInstance(func(name string, _ data.Service) {
		l.send(name)
	}, nil)

	for r := range ch {
		// log.Println(r.Action, r.Node)
		serviceName, _, err := data.DecodePath(r.Node.Key)
		if err != nil {
			log.Println(err)
			continue
		}
		if serviceName == "" {
			// everything deleted; can't cope
			continue
		}
		l.send(serviceName)
	}
}

func (l *Listener) Close() {
	// TODO
}

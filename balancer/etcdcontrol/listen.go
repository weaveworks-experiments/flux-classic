package etcdcontrol

import (
	"net"

	log "github.com/Sirupsen/logrus"
	"golang.org/x/net/context"

	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/data"
	"github.com/weaveworks/flux/common/store"

	"github.com/weaveworks/flux/balancer/model"
)

type Listener struct {
	store     store.Store
	updates   chan<- model.ServiceUpdate
	errorSink daemon.ErrorSink
	context   context.Context
	cancel    context.CancelFunc
	finished  chan struct{}
}

func NewListener(store store.Store, updates chan<- model.ServiceUpdate) daemon.StartFunc {
	return func(es daemon.ErrorSink) daemon.Component {
		ctx, cancel := context.WithCancel(context.Background())
		listener := &Listener{
			store:     store,
			updates:   updates,
			errorSink: es,
			context:   ctx,
			cancel:    cancel,
			finished:  make(chan struct{}),
		}
		go listener.run()
		return listener
	}
}

func (l *Listener) run() {
	defer close(l.finished)
	changes := make(chan data.ServiceChange)
	l.store.WatchServices(l.context, changes, l.errorSink,
		store.QueryServiceOptions{WithInstances: true})

	if err := l.doInitialQuery(); err != nil {
		l.errorSink.Post(err)
		return
	}

	for {
		var change data.ServiceChange
		select {
		case change = <-changes:
		case <-l.context.Done():
			return
		}

		var ms *model.Service
		if !change.ServiceDeleted {
			svc, err := l.store.GetService(change.Name, store.QueryServiceOptions{WithInstances: true})
			if err != nil {
				l.errorSink.Post(err)
				return
			}

			if ms = translateService(svc); ms == nil {
				continue
			}
		}

		select {
		case l.updates <- model.ServiceUpdate{
			Updates: map[string]*model.Service{change.Name: ms},
		}:
		case <-l.context.Done():
			return
		}
	}
}

func (l *Listener) doInitialQuery() error {
	// Send initial state of each service
	svcs, err := l.store.GetAllServices(store.QueryServiceOptions{WithInstances: true})
	if err != nil {
		return err
	}

	updates := make(map[string]*model.Service)
	for _, svc := range svcs {
		if ms := translateService(svc); ms != nil {
			updates[svc.Name] = ms
		}
	}

	select {
	case l.updates <- model.ServiceUpdate{
		Updates: updates,
		Reset:   true,
	}:
	case <-l.context.Done():
	}

	return nil
}

func translateService(svc *store.ServiceInfo) *model.Service {
	var ip net.IP
	if svc.Address != "" {
		ip = net.ParseIP(svc.Address)
		if ip == nil {
			log.Errorf("Bad address \"%s\" for service %s",
				svc.Address, svc.Name)
			return nil
		}
	}

	insts := []model.Instance{}
	for _, instance := range svc.Instances {
		if instance.State != data.LIVE {
			continue // try next instance
		}

		ip := net.ParseIP(instance.Address)
		if ip == nil {
			log.Errorf("Bad address \"%s\" for instance %s/%s",
				instance.Address, svc.Name, instance.Name)
			continue
		}

		insts = append(insts, model.Instance{
			Name:  instance.Name,
			Group: instance.ContainerRule,
			IP:    ip,
			Port:  instance.Port,
		})
	}

	return &model.Service{
		Name:      svc.Name,
		Protocol:  svc.Protocol,
		IP:        ip,
		Port:      svc.Port,
		Instances: insts,
	}
}

func (l *Listener) Stop() {
	l.cancel()
	<-l.finished
}

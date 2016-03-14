package store

import (
	"golang.org/x/net/context"

	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/data"
)

type ServiceUpdate struct {
	Services map[string]*ServiceInfo
	Reset    bool
}

type watchServices struct {
	store     Store
	opts      QueryServiceOptions
	reset     <-chan struct{}
	callback  func(update ServiceUpdate, stop <-chan struct{})
	errorSink daemon.ErrorSink
	context   context.Context
	cancel    context.CancelFunc
	finished  chan struct{}
}

func WatchServicesIndirectStartFunc(store Store, opts QueryServiceOptions, reset <-chan struct{}, cb func(update ServiceUpdate, stop <-chan struct{})) daemon.StartFunc {
	return func(es daemon.ErrorSink) daemon.Component {
		ctx, cancel := context.WithCancel(context.Background())
		ws := &watchServices{
			store:     store,
			opts:      opts,
			reset:     reset,
			callback:  cb,
			errorSink: es,
			context:   ctx,
			cancel:    cancel,
			finished:  make(chan struct{}),
		}
		go func() {
			es.Post(ws.run())
		}()
		return ws
	}
}

func WatchServicesStartFunc(store Store, opts QueryServiceOptions, updates chan<- ServiceUpdate, reset <-chan struct{}) daemon.StartFunc {
	return WatchServicesIndirectStartFunc(store, opts, reset, func(su ServiceUpdate, stop <-chan struct{}) {
		select {
		case updates <- su:
		case <-stop:
		}
	})
}

func (ws *watchServices) run() error {
	defer close(ws.finished)
	changes := make(chan data.ServiceChange)
	ws.store.WatchServices(ws.context, changes, ws.errorSink, ws.opts)

	for {
	drainChanges: // Drain any changes from the WatchServices
		for {
			select {
			case <-changes:
			default:
				break drainChanges
			}
		}

		err := ws.doInitialQuery()
		if err != nil {
			return err
		}

	relayChanges:
		for {
			var change data.ServiceChange
			select {
			case change = <-changes:
			case <-ws.context.Done():
				return nil
			case <-ws.reset:
				break relayChanges
			}

			var svc *ServiceInfo
			if !change.ServiceDeleted {
				if svc, err = ws.store.GetService(change.Name, ws.opts); err != nil {
					return err
				}
			}

			ws.callback(ServiceUpdate{
				Services: map[string]*ServiceInfo{change.Name: svc},
			}, ws.context.Done())
		}
	}
}

func (ws *watchServices) doInitialQuery() error {
	// Send initial state of each service
	svcs, err := ws.store.GetAllServices(ws.opts)
	if err != nil {
		return err
	}

	updates := make(map[string]*ServiceInfo)
	for _, svc := range svcs {
		updates[svc.Name] = svc
	}

drainReset: // Drain any resets
	for {
		select {
		case <-ws.reset:
		default:
			break drainReset
		}
	}

	ws.callback(ServiceUpdate{Services: updates, Reset: true},
		ws.context.Done())
	return nil
}

func (ws *watchServices) Stop() {
	ws.cancel()
	<-ws.finished
}

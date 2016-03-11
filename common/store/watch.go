package store

import (
	"golang.org/x/net/context"

	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/data"
)

type ServicesUpdate struct {
	Updates map[string]*ServiceInfo
	Reset   bool
}

type watchServices struct {
	store     Store
	opts      QueryServiceOptions
	callback  func(update ServicesUpdate, stop <-chan struct{})
	errorSink daemon.ErrorSink
	context   context.Context
	cancel    context.CancelFunc
	finished  chan struct{}
}

func WatchServicesStartFunc(store Store, opts QueryServiceOptions, cb func(update ServicesUpdate, stop <-chan struct{})) daemon.StartFunc {
	return func(es daemon.ErrorSink) daemon.Component {
		ctx, cancel := context.WithCancel(context.Background())
		ws := &watchServices{
			store:     store,
			opts:      opts,
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

func (ws *watchServices) run() error {
	defer close(ws.finished)
	changes := make(chan data.ServiceChange)
	ws.store.WatchServices(ws.context, changes, ws.errorSink, ws.opts)

	err := ws.doInitialQuery()
	if err != nil {
		return err
	}

	for {
		var change data.ServiceChange
		select {
		case change = <-changes:
		case <-ws.context.Done():
			return nil
		}

		var svc *ServiceInfo
		if !change.ServiceDeleted {
			if svc, err = ws.store.GetService(change.Name, ws.opts); err != nil {
				return err
			}
		}

		ws.callback(ServicesUpdate{
			Updates: map[string]*ServiceInfo{change.Name: svc},
		}, ws.context.Done())
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

	ws.callback(ServicesUpdate{Updates: updates, Reset: true},
		ws.context.Done())
	return nil
}

func (ws *watchServices) Stop() {
	ws.cancel()
	<-ws.finished
}

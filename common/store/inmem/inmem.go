package inmem

import (
	"fmt"
	"log"
	"sync"

	"golang.org/x/net/context"

	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/data"
	"github.com/weaveworks/flux/common/store"
)

func NewInMemStore() *InMem {
	return &InMem{
		services:   make(map[string]data.Service),
		groupSpecs: make(map[string]map[string]data.ContainerRule),
		instances:  make(map[string]map[string]data.Instance),
	}
}

type InMem struct {
	services      map[string]data.Service
	groupSpecs    map[string]map[string]data.ContainerRule
	instances     map[string]map[string]data.Instance
	watchersLock  sync.Mutex
	watchers      []watcher
	injectedError error
}

type watcher struct {
	ctx  context.Context
	ch   chan<- data.ServiceChange
	errs daemon.ErrorSink
	opts store.QueryServiceOptions
}

func (w watcher) Done() <-chan struct{} {
	if w.ctx == nil {
		return nil
	}
	return w.ctx.Done()
}

func (s *InMem) fireServiceChange(name string, deleted bool, optsFilter func(store.QueryServiceOptions) bool) {
	ev := data.ServiceChange{Name: name, ServiceDeleted: deleted}

	s.watchersLock.Lock()
	watchers := s.watchers
	s.watchersLock.Unlock()

	for _, watcher := range watchers {
		if optsFilter == nil || optsFilter(watcher.opts) {
			select {
			case watcher.ch <- ev:
			case <-watcher.Done():
			}
		}
	}
}

func (s *InMem) InjectError(err error) {
	s.injectedError = err

	if err != nil {
		// Tell any watchers about the error
		s.watchersLock.Lock()
		defer s.watchersLock.Unlock()

		for _, watcher := range s.watchers {
			watcher.errs.Post(err)
		}
	}
}

func (s *InMem) Ping() error {
	return s.injectedError
}

func (s *InMem) CheckRegisteredService(name string) error {
	if _, found := s.services[name]; !found {
		return fmt.Errorf(`Not found "%s"`, name)
	}
	return s.injectedError
}

func (s *InMem) AddService(name string, svc data.Service) error {
	s.services[name] = svc
	s.groupSpecs[name] = make(map[string]data.ContainerRule)
	s.instances[name] = make(map[string]data.Instance)

	s.fireServiceChange(name, false, nil)
	log.Printf("InMem: service %s updated in store", name)
	return s.injectedError
}

func (s *InMem) RemoveService(name string) error {
	delete(s.services, name)
	delete(s.groupSpecs, name)
	delete(s.instances, name)

	s.fireServiceChange(name, true, nil)
	log.Printf("InMem: service %s removed from store", name)
	return s.injectedError
}

func (s *InMem) RemoveAllServices() error {
	for name, _ := range s.services {
		s.RemoveService(name)
	}
	return s.injectedError
}

func (s *InMem) GetService(name string, opts store.QueryServiceOptions) (*store.ServiceInfo, error) {
	svc, found := s.services[name]
	if !found {
		return nil, fmt.Errorf(`Not found "%s"`, name)
	}

	return s.makeServiceInfo(name, svc, opts), s.injectedError
}

func (s *InMem) makeServiceInfo(name string, svc data.Service, opts store.QueryServiceOptions) *store.ServiceInfo {
	info := &store.ServiceInfo{
		Name:    name,
		Service: svc,
	}

	if opts.WithInstances {
		for n, i := range s.instances[info.Name] {
			info.Instances = append(info.Instances,
				store.InstanceInfo{Name: n, Instance: i})
		}
	}

	if opts.WithContainerRules {
		for n, g := range s.groupSpecs[info.Name] {
			info.ContainerRules = append(info.ContainerRules,
				store.ContainerRuleInfo{Name: n, ContainerRule: g})
		}
	}

	return info
}

func (s *InMem) GetAllServices(opts store.QueryServiceOptions) ([]*store.ServiceInfo, error) {
	var svcs []*store.ServiceInfo

	for name, svc := range s.services {
		svcs = append(svcs, s.makeServiceInfo(name, svc, opts))
	}

	return svcs, s.injectedError
}

func withRuleChanges(opts store.QueryServiceOptions) bool {
	return opts.WithContainerRules
}

func (s *InMem) SetContainerRule(serviceName string, groupName string, spec data.ContainerRule) error {
	groupSpecs, found := s.groupSpecs[serviceName]
	if !found {
		return fmt.Errorf(`Not found "%s"`, serviceName)
	}

	groupSpecs[groupName] = spec
	s.fireServiceChange(serviceName, false, withRuleChanges)
	return s.injectedError
}

func (s *InMem) RemoveContainerRule(serviceName string, groupName string) error {
	groupSpecs, found := s.groupSpecs[serviceName]
	if !found {
		return fmt.Errorf(`Not found "%s"`, serviceName)
	}

	delete(groupSpecs, groupName)
	s.fireServiceChange(serviceName, false, withRuleChanges)
	return s.injectedError
}

func withInstanceChanges(opts store.QueryServiceOptions) bool {
	return opts.WithInstances
}

func (s *InMem) AddInstance(serviceName string, instanceName string, inst data.Instance) error {
	s.instances[serviceName][instanceName] = inst
	s.fireServiceChange(serviceName, false, withInstanceChanges)
	return s.injectedError
}

func (s *InMem) RemoveInstance(serviceName string, instanceName string) error {
	if _, found := s.instances[serviceName][instanceName]; !found {
		return fmt.Errorf("service '%s' has no instance '%s'",
			serviceName, instanceName)
	}

	delete(s.instances[serviceName], instanceName)
	s.fireServiceChange(serviceName, false, withInstanceChanges)
	return s.injectedError
}

func (s *InMem) WatchServices(ctx context.Context, res chan<- data.ServiceChange, errs daemon.ErrorSink, opts store.QueryServiceOptions) {
	if s.injectedError != nil {
		errs.Post(s.injectedError)
		return
	}

	s.watchersLock.Lock()
	defer s.watchersLock.Unlock()
	w := watcher{ctx, res, errs, opts}
	s.watchers = append(s.watchers, w)

	// discard the watcher upon cancellation
	go func() {
		<-w.Done()

		s.watchersLock.Lock()
		defer s.watchersLock.Unlock()
		for i, w := range s.watchers {
			if w.ch == res {
				// need to make a copy
				s.watchers = append(append([]watcher{}, s.watchers[:i]...), s.watchers[i+1:]...)
				break
			}
		}
	}()
}

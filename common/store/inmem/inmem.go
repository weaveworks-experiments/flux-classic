package inmem

import (
	"fmt"
	"log"
	"sync"

	"github.com/squaremo/flux/common/daemon"
	"github.com/squaremo/flux/common/data"
	"github.com/squaremo/flux/common/store"
)

func NewInMemStore() store.Store {
	return &inmem{
		services:   make(map[string]data.Service),
		groupSpecs: make(map[string]map[string]data.ContainerGroupSpec),
		instances:  make(map[string]map[string]data.Instance),
	}
}

type inmem struct {
	services     map[string]data.Service
	groupSpecs   map[string]map[string]data.ContainerGroupSpec
	instances    map[string]map[string]data.Instance
	watchersLock sync.Mutex
	watchers     []watcher
}

type watcher struct {
	ch   chan<- data.ServiceChange
	stop <-chan struct{}
	opts store.WatchServicesOptions
}

func (s *inmem) fireEvent(ev data.ServiceChange, optsFilter func(store.WatchServicesOptions) bool) {
	s.watchersLock.Lock()
	watchers := s.watchers
	s.watchersLock.Unlock()

	for _, watcher := range watchers {
		if optsFilter == nil || optsFilter(watcher.opts) {
			select {
			case watcher.ch <- ev:
			case <-watcher.stop:
			}
		}
	}
}

func (s *inmem) Ping() error {
	return nil
}

func (s *inmem) CheckRegisteredService(name string) error {
	if _, found := s.services[name]; !found {
		return fmt.Errorf(`Not found "%s"`, name)
	}
	return nil
}

func (s *inmem) AddService(name string, svc data.Service) error {
	s.services[name] = svc
	s.groupSpecs[name] = make(map[string]data.ContainerGroupSpec)
	s.instances[name] = make(map[string]data.Instance)

	s.fireEvent(data.ServiceChange{name, false}, nil)
	log.Printf("inmem: service %s updated in store", name)
	return nil
}

func (s *inmem) RemoveService(name string) error {
	delete(s.services, name)
	delete(s.groupSpecs, name)
	delete(s.instances, name)

	s.fireEvent(data.ServiceChange{name, true}, nil)
	log.Printf("inmem: service %s removed from store", name)
	return nil
}

func (s *inmem) RemoveAllServices() error {
	for name, _ := range s.services {
		s.RemoveService(name)
	}
	return nil
}

func (s *inmem) GetService(name string, opts store.QueryServiceOptions) (store.ServiceInfo, error) {
	svc, found := s.services[name]
	if !found {
		return store.ServiceInfo{}, fmt.Errorf(`Not found "%s"`, name)
	}
	info := store.ServiceInfo{
		Name:    name,
		Service: svc,
	}
	s.populateServiceInfo(&info, opts)
	return info, nil
}

func (s *inmem) GetAllServices(opts store.QueryServiceOptions) ([]store.ServiceInfo, error) {
	svcs := []store.ServiceInfo{}
	for n, svc := range s.services {
		info := store.ServiceInfo{
			Name:    n,
			Service: svc,
		}
		s.populateServiceInfo(&info, opts)
		svcs = append(svcs, info)
	}
	return svcs, nil
}

func withGroupSpecChanges(opts store.WatchServicesOptions) bool {
	return opts.WithGroupSpecChanges
}

func (s *inmem) SetContainerGroupSpec(serviceName string, groupName string, spec data.ContainerGroupSpec) error {
	groupSpecs, found := s.groupSpecs[serviceName]
	if !found {
		return fmt.Errorf(`Not found "%s"`, serviceName)
	}

	groupSpecs[groupName] = spec
	s.fireEvent(data.ServiceChange{serviceName, false}, withGroupSpecChanges)
	return nil
}

func (s *inmem) RemoveContainerGroupSpec(serviceName string, groupName string) error {
	groupSpecs, found := s.groupSpecs[serviceName]
	if !found {
		return fmt.Errorf(`Not found "%s"`, serviceName)
	}

	delete(groupSpecs, groupName)
	s.fireEvent(data.ServiceChange{serviceName, false}, withGroupSpecChanges)
	return nil
}

func withInstanceChanges(opts store.WatchServicesOptions) bool {
	return opts.WithInstanceChanges
}

func (s *inmem) AddInstance(serviceName string, instanceName string, inst data.Instance) error {
	s.instances[serviceName][instanceName] = inst
	s.fireEvent(data.ServiceChange{serviceName, false}, withInstanceChanges)
	return nil
}

func (s *inmem) RemoveInstance(serviceName string, instanceName string) error {
	delete(s.instances[serviceName], instanceName)
	s.fireEvent(data.ServiceChange{serviceName, false}, withInstanceChanges)
	return nil
}

func (s *inmem) WatchServices(res chan<- data.ServiceChange, stop <-chan struct{}, _ daemon.ErrorSink, opts store.WatchServicesOptions) {
	s.watchersLock.Lock()
	defer s.watchersLock.Unlock()
	s.watchers = append(s.watchers, watcher{res, stop, opts})

	go func() {
		<-stop

		s.watchersLock.Lock()
		defer s.watchersLock.Unlock()
		for i, w := range s.watchers {
			if w.ch == res {
				// need to make a copy
				s.watchers = append(append([]watcher{}, s.watchers[:i]...), s.watchers[i+1:]...)
			}
		}
	}()
}

// ===

func (s *inmem) populateServiceInfo(info *store.ServiceInfo, opts store.QueryServiceOptions) {
	if opts.WithInstances {
		insts := make([]store.InstanceInfo, 0)
		for n, i := range s.instances[info.Name] {
			insts = append(insts, store.InstanceInfo{
				Name:     n,
				Instance: i,
			})
		}
		info.Instances = insts
	}
	if opts.WithGroupSpecs {
		groups := make([]store.ContainerGroupSpecInfo, 0)
		for n, g := range s.groupSpecs[info.Name] {
			groups = append(groups, store.ContainerGroupSpecInfo{
				Name:               n,
				ContainerGroupSpec: g,
			})
		}
		info.ContainerGroupSpecs = groups
	}
}

package inmem

import (
	"fmt"
	"log"
	"sync"

	"github.com/squaremo/ambergreen/common/data"
	"github.com/squaremo/ambergreen/common/errorsink"
	"github.com/squaremo/ambergreen/common/store"
)

func NewInMemStore() store.Store {
	return &inmem{
		services:  make(map[string]data.Service),
		instances: make(map[string]map[string]data.Instance),
	}
}

type inmem struct {
	services     map[string]data.Service
	instances    map[string]map[string]data.Instance
	watchersLock sync.Mutex
	watchers     []watcher
}

type watcher struct {
	ch   chan<- data.ServiceChange
	stop <-chan struct{}
	wi   bool
}

func (s *inmem) fireEvent(ev data.ServiceChange, isAboutInstance bool) {
	s.watchersLock.Lock()
	watchers := s.watchers
	s.watchersLock.Unlock()

	for _, watcher := range watchers {
		if !isAboutInstance || watcher.wi {
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
	if _, found := s.instances[name]; !found {
		s.instances[name] = make(map[string]data.Instance)
	}
	s.fireEvent(data.ServiceChange{name, false}, false)
	log.Printf("inmem: service %s updated in store", name)
	return nil
}

func (s *inmem) RemoveService(name string) error {
	delete(s.services, name)
	delete(s.instances, name)
	s.fireEvent(data.ServiceChange{name, true}, false)
	log.Printf("inmem: service %s removed from store", name)
	return nil
}

func (s *inmem) RemoveAllServices() error {
	for name, _ := range s.services {
		delete(s.services, name)
		delete(s.instances, name)
		s.fireEvent(data.ServiceChange{name, true}, false)
	}
	return nil
}

func (s *inmem) GetServiceDetails(name string) (data.Service, error) {
	svc, found := s.services[name]
	if !found {
		return data.Service{}, fmt.Errorf(`Not found "%s"`, name)
	}
	return svc, nil
}

func (s *inmem) ForeachServiceInstance(fs store.ServiceFunc, fi store.ServiceInstanceFunc) error {
	for serviceName, svc := range s.services {
		if fs != nil {
			fs(serviceName, svc)
		}
		if fi != nil {
			for instanceName, inst := range s.instances[serviceName] {
				fi(serviceName, instanceName, inst)
			}
		}
	}
	return nil
}

func (s *inmem) AddInstance(serviceName string, instanceName string, inst data.Instance) error {
	s.instances[serviceName][instanceName] = inst
	s.fireEvent(data.ServiceChange{serviceName, false}, true)
	return nil
}

func (s *inmem) RemoveInstance(serviceName string, instanceName string) error {
	delete(s.instances[serviceName], instanceName)
	s.fireEvent(data.ServiceChange{serviceName, false}, true)
	return nil
}

func (s *inmem) ForeachInstance(serviceName string, fi store.InstanceFunc) error {
	for instanceName, inst := range s.instances[serviceName] {
		fi(instanceName, inst)
	}
	return nil
}

func (s *inmem) WatchServices(res chan<- data.ServiceChange, stop <-chan struct{}, _ errorsink.ErrorSink, opts store.WatchServicesOptions) {
	s.watchersLock.Lock()
	defer s.watchersLock.Unlock()
	s.watchers = append(s.watchers, watcher{res, stop, opts.WithInstanceChanges})

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

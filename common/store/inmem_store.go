package store

import (
	"fmt"
	"log"

	"github.com/squaremo/ambergreen/common/data"
	"github.com/squaremo/ambergreen/common/errorsink"
)

func NewInmemStore() Store {
	return &inmem{
		make(map[string]data.Service),
		make(map[string]map[string]data.Instance),
		make([]watcher, 0),
	}
}

type inmem struct {
	services  map[string]data.Service
	instances map[string]map[string]data.Instance
	watchers  []watcher
}

type watcher struct {
	ch chan<- data.ServiceChange
	wi bool
}

func (s *inmem) fireEvent(ev data.ServiceChange, isAboutInstance bool) {
	for _, watcher := range s.watchers {
		if !isAboutInstance || watcher.wi {
			watcher.ch <- ev
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

func (s *inmem) ForeachServiceInstance(fs ServiceFunc, fi ServiceInstanceFunc) error {
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

func (s *inmem) ForeachInstance(serviceName string, fi InstanceFunc) error {
	for instanceName, inst := range s.instances[serviceName] {
		fi(instanceName, inst)
	}
	return nil
}

func (s *inmem) WatchServices(res chan<- data.ServiceChange, stop <-chan struct{}, _ errorsink.ErrorSink, withInstances bool) {
	s.watchers = append(s.watchers, watcher{res, withInstances})
	go func() {
		<-stop
		for i, w := range s.watchers {
			if w.ch == res {
				s.watchers = append(s.watchers[:i], s.watchers[i+1:]...)
			}
		}
	}()
}

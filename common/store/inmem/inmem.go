package inmem

import (
	"fmt"
	"log"
	"sync"
	"time"

	"golang.org/x/net/context"

	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/data"
	"github.com/weaveworks/flux/common/store"
)

func NewInMemStore() store.Store {
	return &inmem{
		services:   make(map[string]data.Service),
		groupSpecs: make(map[string]map[string]data.ContainerRule),
		instances:  make(map[string]map[string]data.Instance),
		hosts:      make(map[string]*data.Host),
		hostTimers: make(map[string]*time.Timer),
	}
}

type inmem struct {
	services     map[string]data.Service
	groupSpecs   map[string]map[string]data.ContainerRule
	instances    map[string]map[string]data.Instance
	hosts        map[string]*data.Host
	hostTimers   map[string]*time.Timer
	watchersLock sync.Mutex
	watchers     []Watcher
}

type Watcher interface {
	Done() <-chan struct{}
}

type watcher struct {
	ctx context.Context
}

func (w watcher) Done() <-chan struct{} {
	if w.ctx == nil {
		return nil
	}
	return w.ctx.Done()
}

type serviceWatcher struct {
	watcher
	ch   chan<- data.ServiceChange
	opts store.QueryServiceOptions
}

type hostWatcher struct {
	watcher
	ch chan<- data.HostChange
}

func (s *inmem) addWatcher(watcher Watcher) {
	s.watchersLock.Lock()
	defer s.watchersLock.Unlock()
	s.watchers = append(s.watchers, watcher)

	// discard the watcher upon cancellation
	go func() {
		<-watcher.Done()

		s.watchersLock.Lock()
		defer s.watchersLock.Unlock()
		for i, w := range s.watchers {
			if w == watcher {
				// need to make a copy
				s.watchers = append(append([]Watcher{}, s.watchers[:i]...), s.watchers[i+1:]...)
				break
			}
		}
	}()
}

func (s *inmem) fireServiceChange(name string, deleted bool, optsFilter func(store.QueryServiceOptions) bool) {
	ev := data.ServiceChange{Name: name, ServiceDeleted: deleted}

	s.watchersLock.Lock()
	watchers := s.watchers
	s.watchersLock.Unlock()

	for _, w := range watchers {
		if watcher, isService := w.(serviceWatcher); isService {
			if optsFilter == nil || optsFilter(watcher.opts) {
				select {
				case watcher.ch <- ev:
				case <-watcher.Done():
				}
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
	s.groupSpecs[name] = make(map[string]data.ContainerRule)
	s.instances[name] = make(map[string]data.Instance)

	s.fireServiceChange(name, false, nil)
	log.Printf("inmem: service %s updated in store", name)
	return nil
}

func (s *inmem) RemoveService(name string) error {
	delete(s.services, name)
	delete(s.groupSpecs, name)
	delete(s.instances, name)

	s.fireServiceChange(name, true, nil)
	log.Printf("inmem: service %s removed from store", name)
	return nil
}

func (s *inmem) RemoveAllServices() error {
	for name, _ := range s.services {
		s.RemoveService(name)
	}
	return nil
}

func (s *inmem) GetService(name string, opts store.QueryServiceOptions) (*store.ServiceInfo, error) {
	svc, found := s.services[name]
	if !found {
		return nil, fmt.Errorf(`Not found "%s"`, name)
	}

	return s.makeServiceInfo(name, svc, opts), nil
}

func (s *inmem) makeServiceInfo(name string, svc data.Service, opts store.QueryServiceOptions) *store.ServiceInfo {
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

func (s *inmem) GetAllServices(opts store.QueryServiceOptions) ([]*store.ServiceInfo, error) {
	var svcs []*store.ServiceInfo

	for name, svc := range s.services {
		svcs = append(svcs, s.makeServiceInfo(name, svc, opts))
	}

	return svcs, nil
}

func withRuleChanges(opts store.QueryServiceOptions) bool {
	return opts.WithContainerRules
}

func (s *inmem) SetContainerRule(serviceName string, groupName string, spec data.ContainerRule) error {
	groupSpecs, found := s.groupSpecs[serviceName]
	if !found {
		return fmt.Errorf(`Not found "%s"`, serviceName)
	}

	groupSpecs[groupName] = spec
	s.fireServiceChange(serviceName, false, withRuleChanges)
	return nil
}

func (s *inmem) RemoveContainerRule(serviceName string, groupName string) error {
	groupSpecs, found := s.groupSpecs[serviceName]
	if !found {
		return fmt.Errorf(`Not found "%s"`, serviceName)
	}

	delete(groupSpecs, groupName)
	s.fireServiceChange(serviceName, false, withRuleChanges)
	return nil
}

func withInstanceChanges(opts store.QueryServiceOptions) bool {
	return opts.WithInstances
}

func (s *inmem) AddInstance(serviceName string, instanceName string, inst data.Instance) error {
	s.instances[serviceName][instanceName] = inst
	s.fireServiceChange(serviceName, false, withInstanceChanges)
	return nil
}

func (s *inmem) RemoveInstance(serviceName string, instanceName string) error {
	if _, found := s.instances[serviceName][instanceName]; !found {
		return fmt.Errorf("service '%s' has no instance '%s'",
			serviceName, instanceName)
	}

	delete(s.instances[serviceName], instanceName)
	s.fireServiceChange(serviceName, false, withInstanceChanges)
	return nil
}

func (s *inmem) WatchServices(ctx context.Context, res chan<- data.ServiceChange, _ daemon.ErrorSink, opts store.QueryServiceOptions) {
	w := serviceWatcher{watcher{ctx}, res, opts}
	s.addWatcher(w)
}

func (s *inmem) GetHosts() ([]*data.Host, error) {
	var hosts []*data.Host = make([]*data.Host, len(s.hosts))
	i := 0
	for _, host := range s.hosts {
		hosts[i] = host
		i++
	}
	return hosts, nil
}

func (s *inmem) Heartbeat(identity string, ttl time.Duration, state *data.Host) error {
	s.hosts[identity] = state
	if s.hostTimers[identity] != nil {
		s.hostTimers[identity].Reset(ttl)
	} else {
		s.hostTimers[identity] = time.AfterFunc(ttl, func() {
			delete(s.hosts, identity)
		})
		s.fireHostChange(identity, false)
	}
	return nil
}

func (s *inmem) DeregisterHost(identity string) error {
	s.deleteHost(identity)
	return nil
}

func (s *inmem) deleteHost(identity string) {
	delete(s.hosts, identity)
	if s.hostTimers[identity] != nil {
		s.hostTimers[identity].Stop()
		delete(s.hostTimers, identity)
	}
	s.fireHostChange(identity, true)
}

func (s *inmem) WatchHosts(ctx context.Context, changes chan<- data.HostChange) {
	w := hostWatcher{watcher{ctx}, changes}
	s.addWatcher(w)
}

func (s *inmem) fireHostChange(identity string, deleted bool) {
	change := data.HostChange{Name: identity, HostDeparted: deleted}
	s.watchersLock.Lock()
	watchers := s.watchers
	s.watchersLock.Unlock()
	for _, w := range watchers {
		if watcher, isHost := w.(hostWatcher); isHost {
			select {
			case watcher.ch <- change:
			case <-watcher.Done():
			}
		}
	}
}

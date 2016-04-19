package inmem

import (
	"fmt"
	"log"
	"sync"
	"time"

	"golang.org/x/net/context"

	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/store"
)

type heartbeat struct {
	updateCount int
}

type sessionHost struct {
	*store.Host
	session string
}

func NewInMem() *InMem {
	return &InMem{
		services:        make(map[string]store.Service),
		groupSpecs:      make(map[string]map[string]store.ContainerRule),
		instances:       make(map[string]map[string]store.Instance),
		hosts:           make(map[string]*sessionHost),
		heartbeats:      make(map[string]*heartbeat),
		heartbeatTimers: make(map[string]*time.Timer),
	}
}

type InMem struct {
	services        map[string]store.Service
	groupSpecs      map[string]map[string]store.ContainerRule
	instances       map[string]map[string]store.Instance
	hosts           map[string]*sessionHost
	heartbeats      map[string]*heartbeat
	heartbeatTimers map[string]*time.Timer
	watchersLock    sync.Mutex
	watchers        []Watcher
	injectedError   error
}

func (s *InMem) GetHeartbeat(identity string) (int, error) {
	if record, found := s.heartbeats[identity]; found {
		return record.updateCount, nil
	}
	return 0, fmt.Errorf(`Host never had heartbeats: "%s"`, identity)
}

type inmemStore struct {
	*InMem
	session string
}

func (inmem *InMem) Store(sessionID string) store.Store {
	return &inmemStore{
		InMem:   inmem,
		session: sessionID,
	}
}

func (s *inmemStore) RegisterHost(identity string, details *store.Host) error {
	s.hosts[identity] = &sessionHost{Host: details, session: s.session}
	s.fireHostChange(identity, false)
	return nil
}

func (s *inmemStore) Heartbeat(ttl time.Duration) error {
	identity := s.session
	fmt.Printf("Heartbeat: %s TTL %d ms\n", identity, ttl/time.Millisecond)

	if record, found := s.heartbeats[identity]; found {
		record.updateCount++
	} else {
		s.heartbeats[identity] = &heartbeat{0}
	}
	if s.heartbeatTimers[identity] != nil {
		s.heartbeatTimers[identity].Reset(ttl)
	} else {
		s.heartbeatTimers[identity] = time.AfterFunc(ttl, func() {
			fmt.Printf("Heartbeat timer fired for %s\n", identity)
			s.EndSession()
		})
	}
	return nil
}

func (s *inmemStore) EndSession() error {
	if timer, found := s.heartbeatTimers[s.session]; found {
		timer.Stop()
		delete(s.heartbeatTimers, s.session)
	}

	for hostName, host := range s.hosts {
		if host.session == s.session {
			delete(s.hosts, hostName)
			s.fireHostChange(hostName, true)
		}
	}
	return nil
}

type Watcher interface {
	Done() <-chan struct{}
	PostError(error)
}

type watcher struct {
	ctx  context.Context
	errs daemon.ErrorSink
}

func (w watcher) Done() <-chan struct{} {
	if w.ctx == nil {
		return nil
	}
	return w.ctx.Done()
}

func (w watcher) PostError(err error) {
	w.errs.Post(err)
}

type serviceWatcher struct {
	watcher
	ch   chan<- store.ServiceChange
	opts store.QueryServiceOptions
}

type hostWatcher struct {
	watcher
	ch chan<- store.HostChange
}

func (s *InMem) addWatcher(watcher Watcher) {
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

func (s *InMem) fireServiceChange(name string, deleted bool, optsFilter func(store.QueryServiceOptions) bool) {
	ev := store.ServiceChange{Name: name, ServiceDeleted: deleted}

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

func (s *InMem) InjectError(err error) {
	s.injectedError = err

	if err != nil {
		// Tell any watchers about the error
		s.watchersLock.Lock()
		defer s.watchersLock.Unlock()

		for _, watcher := range s.watchers {
			watcher.PostError(err)
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

func (s *InMem) AddService(name string, svc store.Service) error {
	s.services[name] = svc
	s.groupSpecs[name] = make(map[string]store.ContainerRule)
	s.instances[name] = make(map[string]store.Instance)

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

func (s *InMem) makeServiceInfo(name string, svc store.Service, opts store.QueryServiceOptions) *store.ServiceInfo {
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

func (s *InMem) SetContainerRule(serviceName string, groupName string, spec store.ContainerRule) error {
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

func (s *InMem) AddInstance(serviceName string, instanceName string, inst store.Instance) error {
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

func (s *InMem) WatchServices(ctx context.Context, res chan<- store.ServiceChange, errs daemon.ErrorSink, opts store.QueryServiceOptions) {
	if s.injectedError != nil {
		errs.Post(s.injectedError)
		return
	}

	w := serviceWatcher{watcher{ctx, errs}, res, opts}
	s.addWatcher(w)
}

func (s *InMem) GetHosts() ([]*store.Host, error) {
	var hosts []*store.Host = make([]*store.Host, len(s.hosts))
	i := 0
	for _, host := range s.hosts {
		hosts[i] = host.Host
		i++
	}
	return hosts, nil
}

func (s *InMem) DeregisterHost(identity string) error {
	s.deleteHost(identity)
	return nil
}

func (s *InMem) deleteHost(identity string) {
	delete(s.hosts, identity)
	s.fireHostChange(identity, true)
}

func (s *InMem) WatchHosts(ctx context.Context, changes chan<- store.HostChange, errs daemon.ErrorSink) {
	w := hostWatcher{watcher{ctx, errs}, changes}
	s.addWatcher(w)
}

func (s *InMem) fireHostChange(identity string, deleted bool) {
	change := store.HostChange{Name: identity, HostDeparted: deleted}
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

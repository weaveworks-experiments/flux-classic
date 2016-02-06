package test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"

	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/data"
	"github.com/weaveworks/flux/common/store"
)

type TestableStore interface {
	store.Store

	Reset(t *testing.T)
}

func RunStoreTestSuite(ts TestableStore, t *testing.T) {
	ts.Reset(t)
	testPing(ts, t)
	ts.Reset(t)
	testServices(ts, t)
	ts.Reset(t)
	testRules(ts, t)
	ts.Reset(t)
	testInstances(ts, t)
	ts.Reset(t)
	testWatchServices(ts, t)
}

func testPing(s store.Store, t *testing.T) {
	require.Nil(t, s.Ping())
}

var testService = data.Service{
	Address:  "1.2.3.4",
	Port:     1234,
	Protocol: "tcp",
}

func testServices(s store.Store, t *testing.T) {
	require.Nil(t, s.AddService("svc", testService))
	svc2, err := s.GetService("svc", store.QueryServiceOptions{})
	require.Nil(t, err)
	require.Equal(t, "svc", svc2.Name)
	require.Equal(t, testService, svc2.Service)

	require.Nil(t, s.CheckRegisteredService("svc"))

	services := func() map[string]data.Service {
		svcs := make(map[string]data.Service)
		ss, err := s.GetAllServices(store.QueryServiceOptions{})
		require.Nil(t, err)
		for _, svc := range ss {
			svcs[svc.Name] = svc.Service
		}
		return svcs
	}

	require.Equal(t, map[string]data.Service{"svc": testService}, services())

	require.Nil(t, s.RemoveService("svc"))
	require.Equal(t, map[string]data.Service{}, services())

	require.Nil(t, s.AddService("svc", testService))
	require.Nil(t, s.RemoveAllServices())
	require.Equal(t, map[string]data.Service{}, services())
}

var testRule = data.ContainerRule{
	Selector: data.Selector{
		"foo": "bar",
	},
}

func testRules(s store.Store, t *testing.T) {
	require.Nil(t, s.AddService("svc", testService))
	require.Nil(t, s.SetContainerRule("svc", "group", testRule))

	svc, err := s.GetService("svc", store.QueryServiceOptions{WithContainerRules: true})
	require.Nil(t, err)

	require.Equal(t, []store.ContainerRuleInfo{
		store.ContainerRuleInfo{
			Name:          "group",
			ContainerRule: testRule,
		},
	}, svc.ContainerRules)

	require.Nil(t, s.RemoveContainerRule("svc", "group"))
	svc, err = s.GetService("svc", store.QueryServiceOptions{WithContainerRules: true})
	require.Nil(t, err)
	require.Empty(t, svc.ContainerRules)
}

var testInst = data.Instance{
	ContainerRule: "group",
	Address:       "1.2.3.4",
	Port:          12345,
	Labels:        map[string]string{"key": "val"},
}

func testInstances(s store.Store, t *testing.T) {
	require.Nil(t, s.AddService("svc", testService))
	require.Nil(t, s.AddInstance("svc", "inst", testInst))

	instances := func() map[string]data.Instance {
		insts := make(map[string]data.Instance)
		require.Nil(t, store.ForeachInstance(s, "svc", func(_, n string, inst data.Instance) error {
			insts[n] = inst
			return nil
		}))
		return insts
	}

	require.Equal(t, map[string]data.Instance{"inst": testInst}, instances())

	serviceInstances := func() map[string]data.Instance {
		insts := make(map[string]data.Instance)
		require.Nil(t, store.ForeachServiceInstance(s, nil, func(sn string, in string, inst data.Instance) error {
			insts[sn+" "+in] = inst
			return nil
		}))
		return insts
	}

	require.Equal(t, map[string]data.Instance{"svc inst": testInst}, serviceInstances())

	require.Nil(t, s.RemoveInstance("svc", "inst"))
	require.Equal(t, map[string]data.Instance{}, instances())
	require.Equal(t, map[string]data.Instance{}, serviceInstances())
}

type watcher struct {
	changes []data.ServiceChange
	stopCh  chan struct{}
	done    chan struct{}
}

func newWatcher(s store.Store, opts store.QueryServiceOptions) *watcher {
	w := &watcher{stopCh: make(chan struct{}), done: make(chan struct{})}
	changes := make(chan data.ServiceChange)

	ctx, cancel := context.WithCancel(context.Background())
	s.WatchServices(ctx, changes, daemon.NewErrorSink(), opts)

	go func() {
		defer close(w.done)
		for {
			select {
			case change := <-changes:
				w.changes = append(w.changes, change)
			case <-w.stopCh:
				cancel()
				return
			}
		}
	}()

	return w
}

func (w *watcher) stop() {
	close(w.stopCh)
	<-w.done
}

func testWatchServices(s store.Store, t *testing.T) {
	check := func(opts store.QueryServiceOptions, body func(w *watcher), changes ...data.ServiceChange) {
		w := newWatcher(s, opts)
		body(w)
		// Yuck.  There's a race between making a change in
		// etcd, and hearing about it via the watch, and I
		// haven't found a nicer way to avoid it.
		time.Sleep(100 * time.Millisecond)
		w.stop()
		require.Equal(t, changes, w.changes)
		require.Nil(t, s.RemoveAllServices())
	}

	check(store.QueryServiceOptions{}, func(w *watcher) {
		require.Nil(t, s.AddService("svc", testService))
	}, data.ServiceChange{"svc", false})

	require.Nil(t, s.AddService("svc", testService))
	check(store.QueryServiceOptions{}, func(w *watcher) {
		require.Nil(t, s.RemoveAllServices())
		require.Nil(t, s.AddService("svc", testService))
		require.Nil(t, s.RemoveService("svc"))
	}, data.ServiceChange{"svc", true}, data.ServiceChange{"svc", false},
		data.ServiceChange{"svc", true})

	// WithInstances false, so adding an instance should not
	// cause an event
	require.Nil(t, s.AddService("svc", testService))
	check(store.QueryServiceOptions{}, func(w *watcher) {
		require.Nil(t, s.AddInstance("svc", "inst", testInst))
	})

	// WithInstances true, so instance changes should
	// cause events
	require.Nil(t, s.AddService("svc", testService))
	check(store.QueryServiceOptions{WithInstances: true},
		func(w *watcher) {
			require.Nil(t, s.AddInstance("svc", "inst", testInst))
			require.Nil(t, s.RemoveInstance("svc", "inst"))
		}, data.ServiceChange{"svc", false},
		data.ServiceChange{"svc", false})

	// WithContainerRules false, so adding a rule should not
	// cause an event
	require.Nil(t, s.AddService("svc", testService))
	check(store.QueryServiceOptions{}, func(w *watcher) {
		require.Nil(t, s.SetContainerRule("svc", "group", testRule))
	})

	// WithContainerRules true, so instance changes should
	// cause events
	require.Nil(t, s.AddService("svc", testService))
	check(store.QueryServiceOptions{WithContainerRules: true},
		func(w *watcher) {
			require.Nil(t, s.SetContainerRule("svc", "group", testRule))
			require.Nil(t, s.RemoveContainerRule("svc", "group"))
		}, data.ServiceChange{"svc", false},
		data.ServiceChange{"svc", false})
}

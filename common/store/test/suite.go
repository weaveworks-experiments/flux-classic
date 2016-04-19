package test

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"

	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/netutil"
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
	ts.Reset(t)
	testHosts(ts, t)
	ts.Reset(t)
	testHostWatch(ts, t)
}

func testPing(s store.Store, t *testing.T) {
	require.Nil(t, s.Ping())
}

var testService = store.Service{
	Address:  &netutil.IPPort{net.ParseIP("1.2.3.4"), 1234},
	Protocol: "tcp",
}

func testServices(s store.Store, t *testing.T) {
	require.Nil(t, s.AddService("svc", testService))
	svc2, err := s.GetService("svc", store.QueryServiceOptions{})
	require.Nil(t, err)
	require.Equal(t, "svc", svc2.Name)
	require.Equal(t, testService, svc2.Service)

	require.Nil(t, s.CheckRegisteredService("svc"))

	services := func() map[string]store.Service {
		svcs := make(map[string]store.Service)
		ss, err := s.GetAllServices(store.QueryServiceOptions{})
		require.Nil(t, err)
		for _, svc := range ss {
			svcs[svc.Name] = svc.Service
		}
		return svcs
	}

	require.Equal(t, map[string]store.Service{"svc": testService}, services())

	require.Nil(t, s.RemoveService("svc"))
	require.Equal(t, map[string]store.Service{}, services())

	require.Nil(t, s.AddService("svc", testService))
	require.Nil(t, s.RemoveAllServices())
	require.Equal(t, map[string]store.Service{}, services())
}

var testRule = store.ContainerRule{
	Selector: store.Selector{
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

var testInst = store.Instance{
	ContainerRule: "group",
	Address:       &netutil.IPPort{net.ParseIP("1.2.3.4"), 12345},
	Labels:        map[string]string{"key": "val"},
}

func testInstances(s store.Store, t *testing.T) {
	require.Nil(t, s.AddService("svc", testService))
	require.Nil(t, s.AddInstance("svc", "inst", testInst))

	instances := func() map[string]store.Instance {
		svc, err := s.GetService("svc", store.QueryServiceOptions{WithInstances: true})
		require.Nil(t, err)

		insts := make(map[string]store.Instance)
		for _, inst := range svc.Instances {
			insts[inst.Name] = inst.Instance
		}
		return insts
	}

	require.Equal(t, map[string]store.Instance{"inst": testInst}, instances())

	serviceInstances := func() map[string]store.Instance {
		svcs, err := s.GetAllServices(store.QueryServiceOptions{WithInstances: true})
		require.Nil(t, err)

		insts := make(map[string]store.Instance)
		for _, svc := range svcs {
			for _, inst := range svc.Instances {
				insts[svc.Name+" "+inst.Name] = inst.Instance
			}
		}
		return insts
	}

	require.Equal(t, map[string]store.Instance{"svc inst": testInst}, serviceInstances())

	require.Nil(t, s.RemoveInstance("svc", "inst"))
	require.Equal(t, map[string]store.Instance{}, instances())
	require.Equal(t, map[string]store.Instance{}, serviceInstances())
}

type watch struct {
	cancel func()
	stopCh chan struct{}
	done   chan struct{}
}

func newWatch(cancel func()) watch {
	return watch{
		cancel: cancel,
		stopCh: make(chan struct{}),
		done:   make(chan struct{}),
	}
}

func (w *watch) stop() {
	close(w.stopCh)
	<-w.done
}

type serviceWatch struct {
	watch
	changes []store.ServiceChange
}

func newServiceWatch(s store.Store, opts store.QueryServiceOptions) *serviceWatch {
	ctx, cancel := context.WithCancel(context.Background())
	w := &serviceWatch{watch: newWatch(cancel)}

	changes := make(chan store.ServiceChange)
	s.WatchServices(ctx, changes, daemon.NewErrorSink(), opts)

	go func() {
		defer close(w.done)
		for {
			select {
			case change := <-changes:
				w.changes = append(w.changes, change)
			case <-w.stopCh:
				w.cancel()
				return
			}
		}
	}()

	return w
}

func testWatchServices(s store.Store, t *testing.T) {
	check := func(opts store.QueryServiceOptions, body func(w *serviceWatch), changes ...store.ServiceChange) {
		w := newServiceWatch(s, opts)
		body(w)
		// Yuck.  There's a race between making a change in
		// etcd, and hearing about it via the watch, and I
		// haven't found a nicer way to avoid it.
		time.Sleep(100 * time.Millisecond)
		w.stop()
		require.Equal(t, changes, w.changes)
		require.Nil(t, s.RemoveAllServices())
	}

	check(store.QueryServiceOptions{}, func(w *serviceWatch) {
		require.Nil(t, s.AddService("svc", testService))
	}, store.ServiceChange{Name: "svc", ServiceDeleted: false})

	require.Nil(t, s.AddService("svc", testService))
	check(store.QueryServiceOptions{}, func(w *serviceWatch) {
		require.Nil(t, s.RemoveAllServices())
		require.Nil(t, s.AddService("svc", testService))
		require.Nil(t, s.RemoveService("svc"))
	}, store.ServiceChange{Name: "svc", ServiceDeleted: true},
		store.ServiceChange{Name: "svc", ServiceDeleted: false},
		store.ServiceChange{Name: "svc", ServiceDeleted: true})

	// WithInstances false, so adding an instance should not
	// cause an event
	require.Nil(t, s.AddService("svc", testService))
	check(store.QueryServiceOptions{}, func(w *serviceWatch) {
		require.Nil(t, s.AddInstance("svc", "inst", testInst))
	})

	// WithInstances true, so instance changes should
	// cause events
	require.Nil(t, s.AddService("svc", testService))
	check(store.QueryServiceOptions{WithInstances: true},
		func(w *serviceWatch) {
			require.Nil(t, s.AddInstance("svc", "inst", testInst))
			require.Nil(t, s.RemoveInstance("svc", "inst"))
		}, store.ServiceChange{Name: "svc", ServiceDeleted: false},
		store.ServiceChange{Name: "svc", ServiceDeleted: false})

	// WithContainerRules false, so adding a rule should not
	// cause an event
	require.Nil(t, s.AddService("svc", testService))
	check(store.QueryServiceOptions{}, func(w *serviceWatch) {
		require.Nil(t, s.SetContainerRule("svc", "group", testRule))
	})

	// WithContainerRules true, so instance changes should
	// cause events
	require.Nil(t, s.AddService("svc", testService))
	check(store.QueryServiceOptions{WithContainerRules: true},
		func(w *serviceWatch) {
			require.Nil(t, s.SetContainerRule("svc", "group", testRule))
			require.Nil(t, s.RemoveContainerRule("svc", "group"))
		}, store.ServiceChange{Name: "svc", ServiceDeleted: false},
		store.ServiceChange{Name: "svc", ServiceDeleted: false})
}

func testHosts(ts TestableStore, t *testing.T) {
	hostID := "foo host"
	hostData := &store.Host{IP: net.ParseIP("192.168.1.65")}
	ts.Heartbeat(10 * time.Second) // hosts depend on the session
	err := ts.RegisterHost(hostID, hostData)
	require.Nil(t, err)
	hosts, err := ts.GetHosts()
	require.Nil(t, err)
	require.Len(t, hosts, 1)
	require.Equal(t, hosts[0], hostData)
	err = ts.DeregisterHost(hostID)
	require.Nil(t, err)
	hosts, err = ts.GetHosts()
	require.Nil(t, err)
	require.Len(t, hosts, 0)

	err = ts.RegisterHost(hostID, hostData)
	require.Nil(t, err)
	hosts, err = ts.GetHosts()
	require.Nil(t, err)
	require.Len(t, hosts, 1)
	require.Equal(t, hosts[0], hostData)
	ts.EndSession()
	hosts, err = ts.GetHosts()
	require.Nil(t, err)
	require.Len(t, hosts, 0)
}

type hostWatch struct {
	watch
	changes []store.HostChange
}

func newHostWatch(s store.Store) *hostWatch {
	ctx, cancel := context.WithCancel(context.Background())
	w := &hostWatch{watch: newWatch(cancel)}

	changes := make(chan store.HostChange)
	s.WatchHosts(ctx, changes, daemon.NewErrorSink())

	go func() {
		defer close(w.done)
		for {
			select {
			case change := <-changes:
				w.changes = append(w.changes, change)
			case <-w.stopCh:
				w.cancel()
				return
			}
		}
	}()

	return w
}

func testHostWatch(ts TestableStore, t *testing.T) {
	check := func(body func(w *hostWatch), changes ...store.HostChange) {
		w := newHostWatch(ts)
		body(w)
		// Yuck, as above.
		time.Sleep(100 * time.Millisecond)
		w.stop()
		require.Equal(t, changes, w.changes)
	}

	hostID := "host number three"
	check(func(w *hostWatch) {
		require.Nil(t, ts.Heartbeat(5*time.Second))
		require.Nil(t, ts.RegisterHost(hostID, &store.Host{IP: net.ParseIP("192.168.3.89")}))
		require.Nil(t, ts.DeregisterHost(hostID))
	}, store.HostChange{hostID, false}, store.HostChange{hostID, true})
	ts.Reset(t)

	hosts, err := ts.GetHosts()
	require.Nil(t, err)
	require.Len(t, hosts, 0)
}

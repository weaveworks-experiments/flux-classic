package etcdstore

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/squaremo/ambergreen/common/data"
	"github.com/squaremo/ambergreen/common/store"
	"github.com/squaremo/ambergreen/common/test/embeddedetcd"
)

func startEtcd(t *testing.T) (*embeddedetcd.SimpleEtcd, store.Store) {
	etcd, err := embeddedetcd.NewSimpleEtcd()
	require.Nil(t, err)
	be := New(fmt.Sprintf("http://localhost:%d", etcd.Port))
	require.Nil(t, be.Ping())
	return etcd, be
}

func TestEtcd(t *testing.T) {
	etcd, be := startEtcd(t)
	defer func() { require.Nil(t, etcd.Destroy()) }()
	require.Nil(t, be.Ping())
}

var testService = data.Service{
	Address:  "1.2.3.4",
	Port:     1234,
	Protocol: "tcp",
	InstanceSpecs: map[data.InstanceGroup]data.InstanceSpec{
		"group": {
			AddressSpec: data.AddressSpec{
				Type: "foo",
				Port: 5678,
			},
			Selector: data.Selector{
				"foo": "bar",
			},
		},
	},
}

func TestServices(t *testing.T) {
	etcd, be := startEtcd(t)
	defer func() { require.Nil(t, etcd.Destroy()) }()

	require.Nil(t, be.AddService("svc", testService))
	svc2, err := be.GetServiceDetails("svc")
	require.Nil(t, err)
	require.Equal(t, testService, svc2)

	require.Nil(t, be.CheckRegisteredService("svc"))

	services := func() map[string]data.Service {
		svcs := make(map[string]data.Service)
		require.Nil(t, be.ForeachServiceInstance(func(n string, s data.Service) {
			svcs[n] = s
		}, nil))
		return svcs
	}

	require.Equal(t, map[string]data.Service{"svc": testService}, services())

	require.Nil(t, be.RemoveService("svc"))
	require.Equal(t, map[string]data.Service{}, services())

	require.Nil(t, be.AddService("svc", testService))
	require.Nil(t, be.RemoveAllServices())
	require.Equal(t, map[string]data.Service{}, services())
}

var testInst = data.Instance{
	InstanceGroup: "group",
	Address:       "1.2.3.4",
	Port:          12345,
	Labels:        map[string]string{"key": "val"},
}

func TestInstances(t *testing.T) {
	etcd, be := startEtcd(t)
	defer func() { require.Nil(t, etcd.Destroy()) }()

	require.Nil(t, be.AddService("svc", testService))
	require.Nil(t, be.AddInstance("svc", "inst", testInst))

	instances := func() map[string]data.Instance {
		insts := make(map[string]data.Instance)
		require.Nil(t, be.ForeachInstance("svc", func(n string, inst data.Instance) {
			insts[n] = inst
		}))
		return insts
	}

	require.Equal(t, map[string]data.Instance{"inst": testInst}, instances())

	serviceInstances := func() map[string]data.Instance {
		insts := make(map[string]data.Instance)
		require.Nil(t, be.ForeachServiceInstance(nil, func(sn string, in string, inst data.Instance) {
			insts[sn+" "+in] = inst
		}))
		return insts
	}

	require.Equal(t, map[string]data.Instance{"svc inst": testInst}, serviceInstances())

	require.Nil(t, be.RemoveInstance("svc", "inst"))
	require.Equal(t, map[string]data.Instance{}, instances())
	require.Equal(t, map[string]data.Instance{}, serviceInstances())
}

type watcher struct {
	changes []data.ServiceChange
	stopCh  chan struct{}
	done    chan struct{}
}

func newWatcher(be store.Store, withInstanceChanges bool) *watcher {
	w := &watcher{stopCh: make(chan struct{}), done: make(chan struct{})}
	changes := make(chan data.ServiceChange)
	stopWatch := make(chan struct{})
	be.WatchServices(changes, stopWatch, withInstanceChanges)
	go func() {
		defer close(w.done)
		for {
			select {
			case change := <-changes:
				w.changes = append(w.changes, change)
			case <-w.stopCh:
				close(stopWatch)
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

func TestWatchServices(t *testing.T) {
	etcd, be := startEtcd(t)
	defer func() { require.Nil(t, etcd.Destroy()) }()

	check := func(withInstanceChanges bool, body func(w *watcher), changes ...data.ServiceChange) {
		w := newWatcher(be, withInstanceChanges)
		body(w)
		// Yuck.  There's a race between making a change in
		// etcd, and hearing about it via the watch, and I
		// haven't found a nicer way to avoid it.
		time.Sleep(100 * time.Millisecond)
		w.stop()
		require.Equal(t, changes, w.changes)
		require.Nil(t, be.RemoveAllServices())
	}

	check(false, func(w *watcher) {
		require.Nil(t, be.AddService("svc", testService))
	}, data.ServiceChange{"svc", false})

	require.Nil(t, be.AddService("svc", testService))
	check(false, func(w *watcher) {
		require.Nil(t, be.RemoveAllServices())
		require.Nil(t, be.AddService("svc", testService))
		require.Nil(t, be.RemoveService("svc"))
	}, data.ServiceChange{"svc", true}, data.ServiceChange{"svc", false},
		data.ServiceChange{"svc", true})

	// withInstanceChanges false, so adding an instance should not
	// cause an event
	require.Nil(t, be.AddService("svc", testService))
	check(false, func(w *watcher) {
		require.Nil(t, be.AddInstance("svc", "inst", testInst))
	})

	// withInstanceChanges true, so instance changes should not
	// cause vents
	require.Nil(t, be.AddService("svc", testService))
	check(true, func(w *watcher) {
		require.Nil(t, be.AddInstance("svc", "inst", testInst))
		require.Nil(t, be.RemoveInstance("svc", "inst"))
	}, data.ServiceChange{"svc", false}, data.ServiceChange{"svc", false})
}

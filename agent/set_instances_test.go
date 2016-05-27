package agent

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/netutil"
	"github.com/weaveworks/flux/common/store"
	"github.com/weaveworks/flux/common/store/inmem"
)

type setInstancesHarness struct {
	hostIP net.IP
	errs   daemon.ErrorSink
	store.Store
	instanceUpdates      chan InstanceUpdate
	instanceUpdatesReset chan struct{}
	didUpdate            chan struct{}
	setInstances         daemon.Component
}

func setupSetInstances(hostIP string) setInstancesHarness {
	h := setInstancesHarness{
		hostIP:               net.ParseIP(hostIP),
		errs:                 daemon.NewErrorSink(),
		Store:                inmem.NewInMem().Store("test session"),
		instanceUpdates:      make(chan InstanceUpdate),
		instanceUpdatesReset: make(chan struct{}, 10),
		didUpdate:            make(chan struct{}),
	}

	h.setInstances = setInstancesConfig{
		hostIP: h.hostIP,
		store:  h.Store,

		instanceUpdates:      h.instanceUpdates,
		instanceUpdatesReset: h.instanceUpdatesReset,
		didUpdate:            h.didUpdate,
	}.StartFunc()(h.errs)

	return h
}

func (h *setInstancesHarness) stop(t *testing.T) {
	h.setInstances.Stop()
	require.Empty(t, h.errs)
}

func makeInstanceUpdate(svc, instName string, inst *store.Instance) InstanceUpdate {
	return InstanceUpdate{
		Instances: map[InstanceKey]*store.Instance{
			InstanceKey{Service: svc, Instance: instName}: inst,
		},
	}
}

func TestSetInstances(t *testing.T) {
	h := setupSetInstances("10.98.99.100")
	h.AddService("svc", store.Service{InstancePort: 80})

	// Add an instance
	inst := store.Instance{
		Host:    store.Host{IP: h.hostIP},
		Address: netutil.ParseIPPortPtr("1.2.3.4:8080"),
	}
	h.instanceUpdates <- makeInstanceUpdate("svc", "inst", &inst)
	<-h.didUpdate

	svc, _ := h.GetService("svc", store.QueryServiceOptions{WithInstances: true})
	require.Len(t, svc.Instances, 1)
	require.Equal(t, inst, svc.Instances["inst"])

	// Remove an instance
	h.instanceUpdates <- makeInstanceUpdate("svc", "inst", nil)
	<-h.didUpdate

	svc, _ = h.GetService("svc", store.QueryServiceOptions{WithInstances: true})
	require.Empty(t, svc.Instances)

	h.stop(t)
}

// Check that stale instances get cleaned up on resets
func TestSetInstancesCleanup(t *testing.T) {
	h := setupSetInstances("10.98.99.100")
	h.AddService("svc", store.Service{InstancePort: 80})

	// Check that the reset signal was sent
	<-h.instanceUpdatesReset

	h.AddInstance("svc", "old-inst", store.Instance{
		Host: store.Host{IP: h.hostIP},
	})

	otherHostInst := store.Instance{
		Host: store.Host{IP: net.ParseIP("10.98.99.101")},
	}
	h.AddInstance("svc", "other-host-inst", otherHostInst)

	inst := store.Instance{
		Host:    store.Host{IP: h.hostIP},
		Address: netutil.ParseIPPortPtr("1.2.3.4:8080"),
	}
	upd := makeInstanceUpdate("svc", "inst", &inst)
	upd.Reset = true
	h.instanceUpdates <- upd
	<-h.didUpdate

	svc, _ := h.GetService("svc", store.QueryServiceOptions{WithInstances: true})
	require.Equal(t, map[string]store.Instance{
		"other-host-inst": otherHostInst,
		"inst":            inst,
	}, svc.Instances)

	h.stop(t)
}

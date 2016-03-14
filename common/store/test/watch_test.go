package test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/data"
	"github.com/weaveworks/flux/common/store"
	"github.com/weaveworks/flux/common/store/inmem"
)

func TestWatchServices(t *testing.T) {
	st := inmem.NewInMemStore()
	st.AddService("foo-svc", data.Service{
		InstancePort: 80,
	})

	updates := make(chan store.ServiceUpdate)
	reset := make(chan struct{})
	es := daemon.NewErrorSink()
	c := store.WatchServicesStartFunc(st, store.QueryServiceOptions{}, updates, reset)(es)

	update := <-updates
	require.True(t, update.Reset)
	require.Len(t, update.Services, 1)
	require.NotNil(t, update.Services["foo-svc"])

	st.AddService("bar-svc", data.Service{
		InstancePort: 80,
	})

	update = <-updates
	require.False(t, update.Reset)
	require.Len(t, update.Services, 1)
	require.NotNil(t, update.Services["bar-svc"])

	// Force a reset
	reset <- struct{}{}
	update = <-updates
	require.True(t, update.Reset)
	require.Len(t, update.Services, 2)
	require.NotNil(t, update.Services["bar-svc"])
	require.NotNil(t, update.Services["foo-svc"])

	st.RemoveService("foo-svc")
	c.Stop()

	// c was stopped, so no updates
	st.RemoveService("bar-svc")
	require.Empty(t, updates)

	require.Empty(t, es)
}

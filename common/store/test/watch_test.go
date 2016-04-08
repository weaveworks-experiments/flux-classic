package test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/store"
	"github.com/weaveworks/flux/common/store/inmem"
)

func TestWatchServices(t *testing.T) {
	st := inmem.NewInMemStore()
	st.AddService("foo-svc", store.Service{
		InstancePort: 80,
	})

	updates := make(chan store.ServiceUpdate)
	es := daemon.NewErrorSink()
	c := store.WatchServicesStartFunc(st, store.QueryServiceOptions{}, updates)(es)

	update := <-updates
	require.True(t, update.Reset)
	require.Len(t, update.Services, 1)
	require.NotNil(t, update.Services["foo-svc"])

	st.AddService("bar-svc", store.Service{
		InstancePort: 80,
	})

	update = <-updates
	require.False(t, update.Reset)
	require.Len(t, update.Services, 1)
	require.NotNil(t, update.Services["bar-svc"])

	st.RemoveService("foo-svc")
	c.Stop()
	st.RemoveService("bar-svc")

	require.Empty(t, es)
}

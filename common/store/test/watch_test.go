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

	updates := make(chan store.ServicesUpdate)
	es := daemon.NewErrorSink()
	c := store.WatchServicesStartFunc(st, store.QueryServiceOptions{}, func(su store.ServicesUpdate, stop <-chan struct{}) {
		select {
		case updates <- su:
		case <-stop:
		}
	})(es)

	update := <-updates
	require.True(t, update.Reset)
	require.Len(t, update.Updates, 1)
	require.NotNil(t, update.Updates["foo-svc"])

	st.AddService("bar-svc", data.Service{
		InstancePort: 80,
	})

	update = <-updates
	require.False(t, update.Reset)
	require.Len(t, update.Updates, 1)
	require.NotNil(t, update.Updates["bar-svc"])

	st.RemoveService("foo-svc")
	c.Stop()
	st.RemoveService("bar-svc")

	require.Empty(t, es)
}

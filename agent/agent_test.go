package agent

import (
	"errors"
	"testing"
	"time"

	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/data"
	"github.com/weaveworks/flux/common/store"
	"github.com/weaveworks/flux/common/store/inmem"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/stretchr/testify/require"
)

// An integration test of all the agent bits

func TestSyncInstancesComponent(t *testing.T) {
	st := inmem.NewInMemStore()
	mdc := newMockDockerClient()

	containerUpdates := make(chan ContainerUpdate)
	containerUpdatesReset := make(chan struct{})
	serviceUpdates := make(chan store.ServiceUpdate)
	serviceUpdatesReset := make(chan struct{})

	conf := SyncInstancesConfig{
		HostIP:  "192.168.11.34",
		Network: LOCAL,
		Store:   st,

		ContainerUpdates:      containerUpdates,
		ContainerUpdatesReset: containerUpdatesReset,
		ServiceUpdates:        serviceUpdates,
		ServiceUpdatesReset:   serviceUpdatesReset,
	}

	dlComp := daemon.SimpleComponent(func(stop <-chan struct{}, errs daemon.ErrorSink) {
		dl := dockerListener{
			client:    mdc,
			stop:      stop,
			errorSink: errs,
		}
		errs.Post(dl.startAux(containerUpdates))
	})

	addService := func(svc string) {
		st.AddService(svc, data.Service{})
		st.SetContainerRule(svc, GROUP, data.ContainerRule{
			Selector: map[string]string{"image": "foo-image"},
		})
		mdc.addContainer(&docker.Container{
			ID: "selected",
			HostConfig: &docker.HostConfig{
				NetworkMode: "default",
			},
			Config: &docker.Config{
				Image: "foo-image:version",
			},
			NetworkSettings: &docker.NetworkSettings{
				IPAddress: "192.168.45.67",
			},
		}, false)
	}

	// Add a service before the agent is running
	addService("svc1")

	es := daemon.NewErrorSink()
	comp := daemon.Aggregate(
		daemon.Reset(containerUpdatesReset,
			daemon.Restart(time.Millisecond, dlComp)),
		daemon.Reset(serviceUpdatesReset,
			daemon.Restart(time.Millisecond,
				store.WatchServicesStartFunc(st,
					store.QueryServiceOptions{WithContainerRules: true},
					serviceUpdates))),
		daemon.Restart(time.Millisecond, conf.StartFunc()))(es)

	// Check that the instance was added appropriately
	time.Sleep(10 * time.Millisecond)
	svc, err := st.GetService("svc1", store.QueryServiceOptions{WithInstances: true})
	require.Nil(t, err)
	require.Len(t, svc.Instances, 1)

	// Simulate a etcd restart
	st.InjectError(errors.New("etcd restarting"))
	time.Sleep(10 * time.Millisecond)
	st.InjectError(nil)
	time.Sleep(10 * time.Millisecond)

	// Add another service
	addService("svc2")
	time.Sleep(10 * time.Millisecond)
	svc, err = st.GetService("svc2", store.QueryServiceOptions{WithInstances: true})
	require.Nil(t, err)
	require.Len(t, svc.Instances, 1)

	comp.Stop()
	require.Empty(t, es)
}

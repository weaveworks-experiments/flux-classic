package agent

import (
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

	es := daemon.NewErrorSink()
	comp := daemon.Aggregate(
		daemon.Reset(containerUpdatesReset,
			daemon.Restart(10*time.Second, dlComp)),
		daemon.Reset(serviceUpdatesReset,
			daemon.Restart(10*time.Second,
				store.WatchServicesStartFunc(st,
					store.QueryServiceOptions{WithContainerRules: true},
					serviceUpdates))),
		daemon.Restart(10*time.Second, conf.StartFunc()))(es)
	time.Sleep(10 * time.Millisecond)

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

	addService("svc1")
	time.Sleep(10 * time.Millisecond)
	svc, err := st.GetService("svc1", store.QueryServiceOptions{WithInstances: true})
	require.Nil(t, err)
	require.Len(t, svc.Instances, 1)

	comp.Stop()
	require.Empty(t, es)
}

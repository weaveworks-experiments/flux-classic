package agent

import (
	"sync"
	"testing"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/stretchr/testify/require"

	"github.com/weaveworks/flux/common/daemon"
)

type mockDockerClient struct {
	lock        sync.Mutex
	containers  map[string]*docker.Container
	listeners   map[chan<- *docker.APIEvents]struct{}
	concAdds    []*docker.Container
	concRemoves []string
}

func newMockDockerClient() *mockDockerClient {
	return &mockDockerClient{
		containers: make(map[string]*docker.Container),
		listeners:  make(map[chan<- *docker.APIEvents]struct{}),
	}
}

func (mdc *mockDockerClient) AddEventListener(listener chan<- *docker.APIEvents) error {
	mdc.lock.Lock()
	defer mdc.lock.Unlock()
	mdc.listeners[listener] = struct{}{}
	return nil
}

func (mdc *mockDockerClient) RemoveEventListener(listener chan *docker.APIEvents) error {
	mdc.lock.Lock()
	defer mdc.lock.Unlock()
	delete(mdc.listeners, listener)
	return nil
}

func (mdc *mockDockerClient) ListContainers(opts docker.ListContainersOptions) ([]docker.APIContainers, error) {
	mdc.lock.Lock()

	var res []docker.APIContainers
	for _, cont := range mdc.containers {
		res = append(res, docker.APIContainers{
			ID:    cont.ID,
			Names: []string{cont.Name},
		})
	}

	mdc.lock.Unlock()

	for _, cont := range mdc.concAdds {
		mdc.addContainer(cont, false)
	}

	for _, id := range mdc.concRemoves {
		mdc.removeContainer(id, false)
	}

	return res, nil
}

func (mdc *mockDockerClient) InspectContainer(id string) (*docker.Container, error) {
	mdc.lock.Lock()
	defer mdc.lock.Unlock()

	res := mdc.containers[id]
	if res == nil {
		return nil, &docker.NoSuchContainer{ID: id}
	}

	return res, nil
}

func (mdc *mockDockerClient) addContainer(cont *docker.Container, async bool) {
	mdc.lock.Lock()
	defer mdc.lock.Unlock()

	mdc.containers[cont.ID] = cont
	mdc.notify(async, docker.APIEvents{
		ID:     cont.ID,
		Status: "start",
	})
}

func (mdc *mockDockerClient) removeContainer(id string, async bool) {
	mdc.lock.Lock()
	defer mdc.lock.Unlock()

	delete(mdc.containers, id)
	mdc.notify(async, docker.APIEvents{
		ID:     id,
		Status: "die",
	})
}

// Generate the events for a container coming and going, but so that
// it is gone by the time it can be inspected
func (mdc *mockDockerClient) transientContainer(id string, async bool) {
	mdc.lock.Lock()
	defer mdc.lock.Unlock()

	mdc.notify(async, docker.APIEvents{
		ID:     id,
		Status: "start",
	}, docker.APIEvents{
		ID:     id,
		Status: "die",
	})
}

func (mdc *mockDockerClient) notify(async bool, evs ...docker.APIEvents) {
	for ch := range mdc.listeners {
		f := func() {
			for i := range evs {
				ch <- &evs[i]
			}
		}
		if async {
			go f()
		} else {
			f()
		}
	}
}

func TestDockerListener(t *testing.T) {
	mdc := newMockDockerClient()
	mdc.addContainer(&docker.Container{
		ID:   "1",
		Name: "/foo",
	}, true)
	mdc.addContainer(&docker.Container{
		ID:   "2",
		Name: "/bar",
	}, true)
	mdc.concRemoves = []string{"2"}
	mdc.concAdds = []*docker.Container{{
		ID:   "3",
		Name: "/qux",
	}}

	updates := make(chan ContainerUpdate)

	errs := daemon.NewErrorSink()
	dlComp := daemon.SimpleComponent(func(stop <-chan struct{}, errs daemon.ErrorSink) {
		dl := dockerListener{
			client:    mdc,
			stop:      stop,
			errorSink: errs,
		}
		errs.Post(dl.startAux(updates))
	})(errs)

	update := <-updates
	require.True(t, update.Reset)
	require.Len(t, update.Containers, 2)
	require.Equal(t, update.Containers["1"].Name, "/foo")
	require.Equal(t, update.Containers["3"].Name, "/qux")

	mdc.removeContainer("1", true)
	update = <-updates
	require.False(t, update.Reset)
	require.Len(t, update.Containers, 1)
	require.Nil(t, update.Containers["1"])

	// This shouldn't result in any updates
	mdc.transientContainer("4", false)

	mdc.addContainer(&docker.Container{
		ID:   "5",
		Name: "/qux",
	}, true)
	update = <-updates
	require.False(t, update.Reset)
	require.Len(t, update.Containers, 1)
	require.Equal(t, update.Containers["5"].Name, "/qux")

	dlComp.Stop()
	require.Empty(t, updates)
	require.Empty(t, errs)
}

func TestDockerListenerEmptyInit(t *testing.T) {
	// Test that a reset update is still sent out even if not
	// containers are present
	mdc := newMockDockerClient()
	updates := make(chan ContainerUpdate)

	errs := daemon.NewErrorSink()
	dlComp := daemon.SimpleComponent(func(stop <-chan struct{}, errs daemon.ErrorSink) {
		dl := dockerListener{
			client:    mdc,
			stop:      stop,
			errorSink: errs,
		}
		errs.Post(dl.startAux(updates))
	})(errs)

	update := <-updates
	require.True(t, update.Reset)
	require.Len(t, update.Containers, 0)

	dlComp.Stop()
	require.Empty(t, updates)
	require.Empty(t, errs)
}

package agent

import (
	"strings"
	"testing"

	"github.com/squaremo/flux/common/daemon"
	"github.com/squaremo/flux/common/data"
	"github.com/squaremo/flux/common/store"
	"github.com/squaremo/flux/common/store/inmem"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/stretchr/testify/require"
)

type mockInspector struct {
	containers map[string]*docker.Container
	events     chan<- *docker.APIEvents
}

func newMockInspector() *mockInspector {
	return &mockInspector{
		containers: make(map[string]*docker.Container),
		events:     nil,
	}
}

type container struct {
	ID          string
	IPAddress   string
	Image       string
	Labels      map[string]string
	Env         map[string]string
	Ports       map[string]string
	NetworkMode string
}

func (m *mockInspector) startContainers(cs ...container) {
	for _, c := range cs {
		env := []string{}
		for k, v := range c.Env {
			env = append(env, strings.Join([]string{k, v}, "="))
		}
		ports := map[docker.Port][]docker.PortBinding{}
		for k, v := range c.Ports {
			ports[docker.Port(k)] = []docker.PortBinding{
				docker.PortBinding{
					HostIP:   "0.0.0.0",
					HostPort: v,
				},
			}
		}

		netmode := c.NetworkMode
		if netmode == "" {
			netmode = "default"
		}
		c1 := &docker.Container{
			ID: c.ID,
			HostConfig: &docker.HostConfig{
				NetworkMode: netmode,
			},
			Config: &docker.Config{
				Image:  c.Image,
				Env:    env,
				Labels: c.Labels,
			},
			NetworkSettings: &docker.NetworkSettings{
				IPAddress: c.IPAddress,
				Ports:     ports,
			},
		}
		m.containers[c.ID] = c1
		if m.events != nil {
			m.events <- &docker.APIEvents{
				Status: "start",
				ID:     c.ID,
			}
		}
	}
}

func (m *mockInspector) stopContainer(ID string) {
	if _, found := m.containers[ID]; found {
		delete(m.containers, ID)
		if m.events != nil {
			m.events <- &docker.APIEvents{
				Status: "die",
				ID:     ID,
			}
		}
	}
}

func (m *mockInspector) InspectContainer(id string) (*docker.Container, error) {
	return m.containers[id], nil
}

func (m *mockInspector) ListContainers(_ docker.ListContainersOptions) ([]docker.APIContainers, error) {
	cs := make([]docker.APIContainers, len(m.containers))
	i := 0
	for _, c := range m.containers {
		cs[i] = docker.APIContainers{
			ID: c.ID,
		}
		i++
	}
	return cs, nil
}

func (m *mockInspector) listenToEvents(events chan<- *docker.APIEvents) {
	m.events = events
}

const GROUP = "deliberately not default"

func addGroup(st store.Store, serviceName string, addr *data.AddressSpec, labels ...string) {
	if len(labels)%2 != 0 {
		panic("Expected key value ... as arguments")
	}
	sel := make(map[string]string)
	for i := 0; i < len(labels); i += 2 {
		sel[labels[i]] = labels[i+1]
	}

	if addr == nil {
		addr = &data.AddressSpec{"fixed", 80}
	}

	st.SetContainerRule(serviceName, GROUP, data.ContainerRule{*addr, sel})
}

func setup(hostIP string) (*Listener, store.Store, *mockInspector) {
	st := inmem.NewInMemStore()
	dc := newMockInspector()
	return NewListener(Config{
		Store:     st,
		HostIP:    hostIP,
		Inspector: dc,
	}), st, dc
}

func TestListenerReconcile(t *testing.T) {
	listener, st, dc := setup("10.98.99.100")
	st.AddService("foo-svc", data.Service{})
	addGroup(st, "foo-svc", nil, "tag", "bobbins", "image", "foo-image")
	st.AddService("bar-svc", data.Service{})
	addGroup(st, "bar-svc", nil, "flux/foo-label", "blorp")
	st.AddService("boo-svc", data.Service{})
	addGroup(st, "boo-svc", nil, "env.SERVICE_NAME", "boo")

	selectedAddress := "192.168.45.67"

	dc.startContainers(container{
		ID:        "selected",
		IPAddress: selectedAddress,
		Image:     "foo-image:bobbins",
		Labels:    map[string]string{"flux/foo-label": "blorp"},
		Env:       map[string]string{"SERVICE_NAME": "boo"},
	}, container{
		ID:        "not",
		IPAddress: "111.111.111.111",
		Image:     "foo-image:not-bobbins",
		Labels:    map[string]string{"flux/foo-label": "something-else"},
		Env:       map[string]string{"SERVICE_NAME": "literally anything"},
	})

	listener.ReadInServices()
	listener.ReadExistingContainers()

	require.Len(t, allInstances(st), 3)
	for _, inst := range allInstances(st) {
		require.Equal(t, selectedAddress, inst.Address)
	}
}

func TestListenerEvents(t *testing.T) {
	listener, st, dc := setup("10.98.90.111")
	// starting condition
	require.Len(t, allInstances(st), 0)

	events := make(chan *docker.APIEvents, 2)
	changes := make(chan data.ServiceChange, 1)

	dc.listenToEvents(events)
	st.WatchServices(nil, changes, daemon.NewErrorSink(),
		store.QueryServiceOptions{WithContainerRules: true})

	// no services defined
	dc.startContainers(container{
		ID:        "foo",
		Image:     "foo-image:latest",
		IPAddress: "192.168.0.67",
	})

	listener.processDockerEvent(<-events)
	require.Len(t, allInstances(st), 0)

	st.AddService("foo-svc", data.Service{})
	listener.processServiceChange(<-changes)
	addGroup(st, "foo-svc", nil, "image", "foo-image")
	listener.processServiceChange(<-changes)
	require.Len(t, allInstances(st), 1)

	addGroup(st, "foo-svc", nil, "image", "not-foo-image")
	listener.processServiceChange(<-changes)
	require.Len(t, allInstances(st), 0)

	dc.startContainers(container{
		ID:        "bar",
		IPAddress: "192.168.34.87",
		Image:     "not-foo-image:version",
	}, container{
		ID:        "baz",
		IPAddress: "192.168.34.99",
		Image:     "not-foo-image:version2",
	})
	listener.processDockerEvent(<-events)
	listener.processDockerEvent(<-events)
	require.Len(t, allInstances(st), 2)

	dc.stopContainer("baz")
	listener.processDockerEvent(<-events)
	require.Len(t, allInstances(st), 1)

	st.RemoveService("foo-svc")
	listener.processServiceChange(<-changes)
	require.Len(t, allInstances(st), 0)
}

func allInstances(st store.Store) []data.Instance {
	res := make([]data.Instance, 0)
	store.ForeachServiceInstance(st, nil, func(_, _ string, inst data.Instance) error {
		res = append(res, inst)
		return nil
	})
	return res
}

func TestMappedPort(t *testing.T) {
	listener, st, dc := setup("11.98.99.98")

	st.AddService("blorp-svc", data.Service{})
	addGroup(st, "blorp-svc", &data.AddressSpec{
		Type: data.MAPPED,
		Port: 8080,
	}, "image", "blorp-image")
	dc.startContainers(container{
		ID:        "blorp-instance",
		IPAddress: "10.13.14.15",
		Image:     "blorp-image:tag",
		Ports: map[string]string{
			"8080/tcp": "3456",
		},
	})

	listener.ReadInServices()
	listener.ReadExistingContainers()

	require.Len(t, allInstances(st), 1)
	store.ForeachInstance(st, "blorp-svc", func(_, _ string, inst data.Instance) error {
		require.Equal(t, listener.hostIP, inst.Address)
		require.Equal(t, 3456, inst.Port)
		require.Equal(t, data.LIVE, inst.State)
		return nil
	})
}

func TestNoAddress(t *testing.T) {
	listener, st, dc := setup("192.168.3.4")

	st.AddService("important-svc", data.Service{})
	addGroup(st, "important-svc", &data.AddressSpec{
		Type: data.MAPPED,
		Port: 8080,
	}, "image", "important-image")
	dc.startContainers(container{
		ID:        "oops-instance",
		IPAddress: "10.13.14.15",
		Image:     "important-image:greatest",
		// No published port
	})

	listener.ReadInServices()
	listener.ReadExistingContainers()

	require.Len(t, allInstances(st), 1)
	store.ForeachInstance(st, "blorp-svc", func(_, _ string, inst data.Instance) error {
		require.Equal(t, listener.hostIP, inst.Address)
		require.Equal(t, 3456, inst.Port)
		require.Equal(t, data.NOADDR, inst.State)
		return nil
	})
}

func TestHostNetworking(t *testing.T) {
	listener, st, dc := setup("192.168.5.135")

	st.AddService("blorp-svc", data.Service{})
	addGroup(st, "blorp-svc", &data.AddressSpec{
		Type: data.FIXED,
		Port: 8080,
	}, "image", "blorp-image")
	dc.startContainers(container{
		NetworkMode: "host",
		ID:          "blorp-instance",
		IPAddress:   "",
		Image:       "blorp-image:tag",
	})

	listener.ReadInServices()
	listener.ReadExistingContainers()

	require.Len(t, allInstances(st), 1)
	store.ForeachInstance(st, "blorp-svc", func(_, _ string, inst data.Instance) error {
		require.Equal(t, listener.hostIP, inst.Address)
		require.Equal(t, 8080, inst.Port)
		require.Equal(t, data.LIVE, inst.State)
		return nil
	})
}

func TestOtherHostsEntries(t *testing.T) {
	listener1, st, dc1 := setup("192.168.11.34")
	dc2 := newMockInspector()
	listener2 := NewListener(Config{
		Store:     st,
		HostIP:    "192.168.11.5",
		Inspector: dc2,
	})

	st.AddService("foo-svc", data.Service{})
	addGroup(st, "foo-svc", nil, "image", "foo-image")
	dc1.startContainers(container{
		ID:        "bar1",
		IPAddress: "192.168.34.1",
		Image:     "foo-image:version",
	}, container{
		ID:        "baz1",
		IPAddress: "192.168.34.2",
		Image:     "foo-image:version2",
	})
	dc2.startContainers(container{
		ID:        "bar2",
		IPAddress: "192.168.34.3",
		Image:     "foo-image:version",
	}, container{
		ID:        "baz2",
		IPAddress: "192.168.34.4",
		Image:     "foo-image:version2",
	})
	// let listener on the first host add its instances
	listener1.ReadInServices()
	listener1.ReadExistingContainers()

	require.Len(t, allInstances(st), 2)

	// let listener on the second host add its instances
	listener2.ReadInServices()
	listener2.ReadExistingContainers()

	require.Len(t, allInstances(st), 4)

	// simulate an agent restart; in the meantime, a container has
	// stopped.
	dc2.stopContainer("baz2")

	// NB: the Read* methods assume once-only execution, on startup.
	listener2 = NewListener(Config{
		Store:     st,
		HostIP:    "192.168.11.5",
		Inspector: dc2,
	})
	listener2.ReadExistingContainers()
	listener2.ReadInServices()

	require.Len(t, allInstances(st), 3)
}

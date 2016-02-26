package agent

import (
	"strings"
	"testing"

	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/data"
	"github.com/weaveworks/flux/common/store"
	"github.com/weaveworks/flux/common/store/inmem"

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

func addGroup(st store.Store, serviceName string, labels ...string) {
	if len(labels)%2 != 0 {
		panic("Expected key value ... as arguments")
	}
	sel := make(map[string]string)
	for i := 0; i < len(labels); i += 2 {
		sel[labels[i]] = labels[i+1]
	}

	st.SetContainerRule(serviceName, GROUP,
		data.ContainerRule{Selector: sel})
}

func setup(hostIP, netmode string) (*Listener, store.Store, *mockInspector) {
	st := inmem.NewInMemStore()
	dc := newMockInspector()
	if netmode == "" {
		netmode = LOCAL
	}
	return NewListener(Config{
		Store:     st,
		Network:   netmode,
		HostIP:    hostIP,
		Inspector: dc,
	}), st, dc
}

func TestListenerReconcile(t *testing.T) {
	listener, st, dc := setup("10.98.99.100", GLOBAL)
	st.AddService("foo-svc", data.Service{
		InstancePort: 80,
	})
	addGroup(st, "foo-svc", "tag", "bobbins", "image", "foo-image")
	st.AddService("bar-svc", data.Service{
		InstancePort: 80,
	})
	addGroup(st, "bar-svc", "flux/foo-label", "blorp")
	st.AddService("boo-svc", data.Service{
		InstancePort: 80,
	})
	addGroup(st, "boo-svc", "env.SERVICE_NAME", "boo")

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

	insts := allInstances(st, t)
	require.Len(t, insts, 3)
	for _, inst := range insts {
		require.Equal(t, selectedAddress, inst.Address)
	}
}

func TestListenerEvents(t *testing.T) {
	listener, st, dc := setup("10.98.90.111", "")
	// starting condition
	require.Len(t, allInstances(st, t), 0)

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
	require.Len(t, allInstances(st, t), 0)

	st.AddService("foo-svc", data.Service{})
	listener.processServiceChange(<-changes)
	addGroup(st, "foo-svc", "image", "foo-image")
	listener.processServiceChange(<-changes)
	require.Len(t, allInstances(st, t), 1)

	addGroup(st, "foo-svc", "image", "not-foo-image")
	listener.processServiceChange(<-changes)
	require.Len(t, allInstances(st, t), 0)

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
	require.Len(t, allInstances(st, t), 2)

	dc.stopContainer("baz")
	listener.processDockerEvent(<-events)
	require.Len(t, allInstances(st, t), 1)

	st.RemoveService("foo-svc")
	listener.processServiceChange(<-changes)
	require.Len(t, allInstances(st, t), 0)
}

func allInstances(st store.Store, t *testing.T) []data.Instance {
	var res []data.Instance
	svcs, err := st.GetAllServices(store.QueryServiceOptions{WithInstances: true})
	require.Nil(t, err)
	for _, svc := range svcs {
		for _, inst := range svc.Instances {
			res = append(res, inst.Instance)
		}
	}
	return res
}

func TestMappedPort(t *testing.T) {
	listener, st, dc := setup("11.98.99.98", LOCAL)

	st.AddService("blorp-svc", data.Service{
		InstancePort: 8080,
	})
	addGroup(st, "blorp-svc", "image", "blorp-image")
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

	require.Len(t, allInstances(st, t), 1)
	svc, err := st.GetService("blorp-svc", store.QueryServiceOptions{WithInstances: true})
	require.Nil(t, err)
	require.Equal(t, listener.hostIP, svc.Instances[0].Address)
	require.Equal(t, 3456, svc.Instances[0].Port)
	require.Equal(t, data.LIVE, svc.Instances[0].State)
}

func TestMultihostNetworking(t *testing.T) {
	instAddress := "10.13.14.15"
	instPort := 8080

	listener, st, dc := setup("11.98.99.98", GLOBAL)

	st.AddService("blorp-svc", data.Service{
		InstancePort: instPort,
	})
	addGroup(st, "blorp-svc", "image", "blorp-image")
	dc.startContainers(container{
		ID:        "blorp-instance",
		IPAddress: instAddress,
		Image:     "blorp-image:tag",
		Ports: map[string]string{
			"8080/tcp": "3456",
		},
	})

	listener.ReadInServices()
	listener.ReadExistingContainers()

	require.Len(t, allInstances(st, t), 1)
	svc, err := st.GetService("blorp-svc", store.QueryServiceOptions{WithInstances: true})
	require.Nil(t, err)
	require.Equal(t, instAddress, svc.Instances[0].Address)
	require.Equal(t, instPort, svc.Instances[0].Port)
	require.Equal(t, data.LIVE, svc.Instances[0].State)
}

func TestNoAddress(t *testing.T) {
	listener, st, dc := setup("192.168.3.4", LOCAL)

	st.AddService("important-svc", data.Service{
		InstancePort: 80,
	})
	addGroup(st, "important-svc", "image", "important-image")
	dc.startContainers(container{
		ID:        "oops-instance",
		IPAddress: "10.13.14.15",
		Image:     "important-image:greatest",
		// No published port
	})

	listener.ReadInServices()
	listener.ReadExistingContainers()

	require.Len(t, allInstances(st, t), 1)
	svc, err := st.GetService("important-svc", store.QueryServiceOptions{WithInstances: true})
	require.Nil(t, err)
	require.Equal(t, "", svc.Instances[0].Address)
	require.Equal(t, 0, svc.Instances[0].Port)
	require.Equal(t, data.NOADDR, svc.Instances[0].State)
}

func TestHostNetworking(t *testing.T) {
	listener, st, dc := setup("192.168.5.135", GLOBAL)

	st.AddService("blorp-svc", data.Service{
		InstancePort: 8080,
	})
	addGroup(st, "blorp-svc", "image", "blorp-image")
	dc.startContainers(container{
		NetworkMode: "host",
		ID:          "blorp-instance",
		IPAddress:   "",
		Image:       "blorp-image:tag",
	})

	listener.ReadInServices()
	listener.ReadExistingContainers()

	require.Len(t, allInstances(st, t), 1)
	svc, err := st.GetService("blorp-svc", store.QueryServiceOptions{WithInstances: true})
	require.Nil(t, err)
	require.Equal(t, listener.hostIP, svc.Instances[0].Address)
	require.Equal(t, 8080, svc.Instances[0].Port)
	require.Equal(t, data.LIVE, svc.Instances[0].State)
}

func TestOtherHostsEntries(t *testing.T) {
	listener1, st, dc1 := setup("192.168.11.34", LOCAL)
	dc2 := newMockInspector()
	listener2 := NewListener(Config{
		Store:     st,
		HostIP:    "192.168.11.5",
		Inspector: dc2,
	})

	st.AddService("foo-svc", data.Service{})
	addGroup(st, "foo-svc", "image", "foo-image")
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

	require.Len(t, allInstances(st, t), 2)

	// let listener on the second host add its instances
	listener2.ReadInServices()
	listener2.ReadExistingContainers()

	require.Len(t, allInstances(st, t), 4)

	// simulate an agent restart; in the meantime, a container has
	// stopped.
	dc2.stopContainer("baz2")

	// NB: the Read* methods assume once-only execution, on startup.
	listener2 = NewListener(Config{
		Store:     st,
		Network:   LOCAL,
		HostIP:    "192.168.11.5",
		Inspector: dc2,
	})
	listener2.ReadExistingContainers()
	listener2.ReadInServices()

	require.Len(t, allInstances(st, t), 3)
}

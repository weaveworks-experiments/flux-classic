package main

import (
	"testing"

	"github.com/squaremo/ambergreen/common/data"
	"github.com/squaremo/ambergreen/common/store"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/stretchr/testify/require"
)

type mockInspector struct {
	containers map[string]*docker.Container
}

func newMockInspector() *mockInspector {
	return &mockInspector{
		containers: make(map[string]*docker.Container),
	}
}

func (m *mockInspector) addContainer(cs ...*docker.Container) {
	for _, c := range cs {
		m.containers[c.ID] = c
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
	}
	return cs, nil
}

func serviceFromSel(labels ...string) data.Service {
	if len(labels)%2 != 0 {
		panic("Expected key value ... as arguments")
	}
	sel := make(map[string]string)
	for i := 0; i < len(labels); i += 2 {
		sel[labels[i]] = labels[i+1]
	}
	return data.Service{
		InstanceSpecs: map[data.InstanceGroup]data.InstanceSpec{
			"default": data.InstanceSpec{
				data.AddressSpec{"fixed", 80},
				sel,
			},
		},
	}
}

func TestListener(t *testing.T) {
	st := store.NewInmemStore()
	dc := newMockInspector()
	config := Config{
		Store:     st,
		HostIP:    "10.98.99.100",
		Inspector: dc,
	}
	listener := NewListener(config)
	listener.ReadInServices()
	listener.ReadExistingContainers()
	events := make(chan *docker.APIEvents, 0)
	listener.Run(events)

	require.Equal(t, 0, len(allInstances(st)))

	// no services defined
	dc.addContainer(&docker.Container{
		ID: "foobar",
		Config: &docker.Config{
			Image: "foo-image:latest",
		},
		NetworkSettings: &docker.NetworkSettings{
			IPAddress: "192.168.0.67",
		},
	})

	events <- &docker.APIEvents{
		Status: "start",
		ID:     "foobar",
	}
	require.Equal(t, 0, len(allInstances(st)))

	st.AddService("foo-svc", serviceFromSel("image", "foo-image"))
	listener.wait()
	require.Equal(t, 1, len(allInstances(st)))
}

func allInstances(st store.Store) []data.Instance {
	res := make([]data.Instance, 0)
	st.ForeachServiceInstance(nil, func(_, _ string, inst data.Instance) {
		res = append(res, inst)
	})
	return res
}

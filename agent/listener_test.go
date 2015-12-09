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

type container struct {
	ID        string
	IPAddress string
	Image     string
}

func (m *mockInspector) startContainers(cs ...container) {
	for _, c := range cs {
		c1 := &docker.Container{
			ID: c.ID,
			Config: &docker.Config{
				Image: c.Image,
			},
			NetworkSettings: &docker.NetworkSettings{
				IPAddress: c.IPAddress,
			},
		}
		m.containers[c.ID] = c1
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
	// starting condition
	require.Len(t, allInstances(st), 0)

	// no services defined
	dc.startContainers(container{
		ID:        "foo",
		Image:     "foo-image:latest",
		IPAddress: "192.168.0.67",
	})
	listener.containerStarted("foo")
	require.Len(t, allInstances(st), 0)

	st.AddService("foo-svc", serviceFromSel("image", "foo-image"))
	listener.serviceUpdated("foo-svc")
	require.Len(t, allInstances(st), 1)

	st.AddService("foo-svc", serviceFromSel("image", "not-foo-image"))
	listener.serviceUpdated("foo-svc")
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
	listener.containerStarted("bar")
	listener.containerStarted("baz")
	require.Len(t, allInstances(st), 2)

	st.RemoveService("foo-svc")
	listener.serviceRemoved("foo-svc")
	require.Len(t, allInstances(st), 0)
}

func allInstances(st store.Store) []data.Instance {
	res := make([]data.Instance, 0)
	st.ForeachServiceInstance(nil, func(_, _ string, inst data.Instance) {
		res = append(res, inst)
	})
	return res
}

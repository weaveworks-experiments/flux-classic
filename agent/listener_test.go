package main

import (
	"strings"
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
	Labels    map[string]string
	Env       map[string]string
}

func (m *mockInspector) startContainers(cs ...container) {
	for _, c := range cs {
		env := []string{}
		for k, v := range c.Env {
			env = append(env, strings.Join([]string{k, v}, "="))
		}
		c1 := &docker.Container{
			ID: c.ID,
			Config: &docker.Config{
				Image:  c.Image,
				Env:    env,
				Labels: c.Labels,
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
		i++
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

func setup() (*Listener, store.Store, *mockInspector) {
	st := store.NewInmemStore()
	dc := newMockInspector()
	return NewListener(Config{
		Store:     st,
		HostIP:    "10.98.99.100",
		Inspector: dc,
	}), st, dc
}

func TestListenerReconcile(t *testing.T) {
	listener, st, dc := setup()
	st.AddService("foo-svc", serviceFromSel("tag", ":bobbins", "image", "foo-image"))
	st.AddService("bar-svc", serviceFromSel("amber/foo-label", "blorp"))
	st.AddService("boo-svc", serviceFromSel("env.SERVICE_NAME", "boo"))

	selectedAddress := "192.168.45.67"

	dc.startContainers(container{
		ID:        "selected",
		IPAddress: selectedAddress,
		Image:     "foo-image:bobbins",
		Labels:    map[string]string{"amber/foo-label": "blorp"},
		Env:       map[string]string{"SERVICE_NAME": "boo"},
	}, container{
		ID:        "not",
		IPAddress: "111.111.111.111",
		Image:     "foo-image:not-bobbins",
		Labels:    map[string]string{"amber/foo-label": "something-else"},
		Env:       map[string]string{"SERVICE_NAME": "literally anything"},
	})

	listener.ReadInServices()
	listener.ReadExistingContainers()
	listener.reconcile()

	require.Len(t, allInstances(st), 3)
	for _, inst := range allInstances(st) {
		require.Equal(t, selectedAddress, inst.Address)
	}
}

func TestListenerEvents(t *testing.T) {
	listener, st, dc := setup()
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

	listener.containerDied("baz")
	require.Len(t, allInstances(st), 1)

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

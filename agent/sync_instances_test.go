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

type container struct {
	ID          string
	IPAddress   string
	Image       string
	Labels      map[string]string
	Env         map[string]string
	Ports       map[string]string
	NetworkMode string
}

func resetContainers(cs ...container) ContainerUpdate {
	return ContainerUpdate{
		Containers: makeContainersMap(cs),
		Reset:      true,
	}
}

func addContainer(c container) ContainerUpdate {
	return ContainerUpdate{
		Containers: makeContainersMap([]container{c}),
	}
}

func removeContainer(id string) ContainerUpdate {
	return ContainerUpdate{
		Containers: map[string]*docker.Container{id: nil},
	}
}

func makeContainersMap(cs []container) map[string]*docker.Container {
	containers := make(map[string]*docker.Container)

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
		containers[c.ID] = c1
	}

	return containers
}

const GROUP = "deliberately not default"

type harness struct {
	si *SyncInstances
	store.Store
	es               daemon.ErrorSink
	serviceUpdates   chan store.ServiceUpdate
	watchingServices daemon.Component
}

func setup(st store.Store, hostIP, netmode string) (h harness) {
	if st == nil {
		st = inmem.NewInMemStore()
	}

	if netmode == "" {
		netmode = LOCAL
	}

	h.Store = st
	h.si = NewSyncInstances(Config{
		Store:   h.Store,
		Network: netmode,
		HostIP:  hostIP,
	})
	h.es = daemon.NewErrorSink()
	h.serviceUpdates = make(chan store.ServiceUpdate)
	return
}

func (h *harness) watchServices() {
	h.watchingServices = store.WatchServicesStartFunc(h.Store,
		store.QueryServiceOptions{WithContainerRules: true},
		h.serviceUpdates)(h.es)
}

func (h *harness) stop(t *testing.T) {
	h.watchingServices.Stop()
	require.Empty(t, h.es)
}

func (h *harness) addGroup(serviceName string, labels ...string) {
	if len(labels)%2 != 0 {
		panic("Expected key value ... as arguments")
	}
	sel := make(map[string]string)
	for i := 0; i < len(labels); i += 2 {
		sel[labels[i]] = labels[i+1]
	}

	h.SetContainerRule(serviceName, GROUP,
		data.ContainerRule{Selector: sel})
}

func (h *harness) allInstances(t *testing.T) []data.Instance {
	var res []data.Instance
	svcs, err := h.GetAllServices(store.QueryServiceOptions{WithInstances: true})
	require.Nil(t, err)
	for _, svc := range svcs {
		for _, inst := range svc.Instances {
			res = append(res, inst.Instance)
		}
	}
	return res
}

func TestSyncInstancesReconcile(t *testing.T) {
	h := setup(nil, "10.98.99.100", GLOBAL)
	h.AddService("foo-svc", data.Service{
		InstancePort: 80,
	})
	h.addGroup("foo-svc", "tag", "bobbins", "image", "foo-image")
	h.AddService("bar-svc", data.Service{
		InstancePort: 80,
	})
	h.addGroup("bar-svc", "flux/foo-label", "blorp")
	h.AddService("boo-svc", data.Service{
		InstancePort: 80,
	})
	h.addGroup("boo-svc", "env.SERVICE_NAME", "boo")

	selectedAddress := "192.168.45.67"

	h.watchServices()
	h.si.processServiceUpdate(<-h.serviceUpdates)
	h.si.processContainerUpdate(resetContainers(container{
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
	}))

	insts := h.allInstances(t)
	require.Len(t, insts, 3)
	for _, inst := range insts {
		require.Equal(t, selectedAddress, inst.Address)
	}

	h.stop(t)
}

func TestSyncInstancesEvents(t *testing.T) {
	h := setup(nil, "10.98.90.111", "")
	// starting condition
	require.Len(t, h.allInstances(t), 0)

	// no services defined
	h.watchServices()
	h.si.processServiceUpdate(<-h.serviceUpdates)
	h.si.processContainerUpdate(resetContainers(container{
		ID:        "foo",
		Image:     "foo-image:latest",
		IPAddress: "192.168.0.67",
	}))
	require.Len(t, h.allInstances(t), 0)

	h.AddService("foo-svc", data.Service{})
	h.si.processServiceUpdate(<-h.serviceUpdates)
	h.addGroup("foo-svc", "image", "foo-image")
	h.si.processServiceUpdate(<-h.serviceUpdates)
	require.Len(t, h.allInstances(t), 1)

	h.addGroup("foo-svc", "image", "not-foo-image")
	h.si.processServiceUpdate(<-h.serviceUpdates)
	require.Len(t, h.allInstances(t), 0)

	h.si.processContainerUpdate(addContainer(container{
		ID:        "bar",
		IPAddress: "192.168.34.87",
		Image:     "not-foo-image:version",
	}))
	h.si.processContainerUpdate(addContainer(container{
		ID:        "baz",
		IPAddress: "192.168.34.99",
		Image:     "not-foo-image:version2",
	}))
	require.Len(t, h.allInstances(t), 2)

	h.si.processContainerUpdate(removeContainer("baz"))
	require.Len(t, h.allInstances(t), 1)

	h.RemoveService("foo-svc")
	h.si.processServiceUpdate(<-h.serviceUpdates)
	require.Len(t, h.allInstances(t), 0)
	h.stop(t)
}

func TestMappedPort(t *testing.T) {
	h := setup(nil, "11.98.99.98", LOCAL)

	h.AddService("blorp-svc", data.Service{
		InstancePort: 8080,
	})
	h.addGroup("blorp-svc", "image", "blorp-image")

	h.si.processContainerUpdate(resetContainers(container{
		ID:        "blorp-instance",
		IPAddress: "10.13.14.15",
		Image:     "blorp-image:tag",
		Ports: map[string]string{
			"8080/tcp": "3456",
		},
	}))
	h.watchServices()
	h.si.processServiceUpdate(<-h.serviceUpdates)

	require.Len(t, h.allInstances(t), 1)
	svc, err := h.GetService("blorp-svc", store.QueryServiceOptions{WithInstances: true})
	require.Nil(t, err)
	require.Equal(t, h.si.hostIP, svc.Instances[0].Address)
	require.Equal(t, 3456, svc.Instances[0].Port)
	require.Equal(t, data.LIVE, svc.Instances[0].State)
	h.stop(t)
}

func TestMultihostNetworking(t *testing.T) {
	instAddress := "10.13.14.15"
	instPort := 8080

	h := setup(nil, "11.98.99.98", GLOBAL)

	h.AddService("blorp-svc", data.Service{
		InstancePort: instPort,
	})
	h.addGroup("blorp-svc", "image", "blorp-image")

	h.si.processContainerUpdate(resetContainers(container{
		ID:        "blorp-instance",
		IPAddress: instAddress,
		Image:     "blorp-image:tag",
		Ports: map[string]string{
			"8080/tcp": "3456",
		},
	}))
	h.watchServices()
	h.si.processServiceUpdate(<-h.serviceUpdates)

	require.Len(t, h.allInstances(t), 1)
	svc, err := h.GetService("blorp-svc", store.QueryServiceOptions{WithInstances: true})
	require.Nil(t, err)
	require.Equal(t, instAddress, svc.Instances[0].Address)
	require.Equal(t, instPort, svc.Instances[0].Port)
	require.Equal(t, data.LIVE, svc.Instances[0].State)
	h.stop(t)
}

func TestNoAddress(t *testing.T) {
	h := setup(nil, "192.168.3.4", LOCAL)

	h.AddService("important-svc", data.Service{
		InstancePort: 80,
	})
	h.addGroup("important-svc", "image", "important-image")

	h.si.processContainerUpdate(resetContainers(container{
		ID:        "oops-instance",
		IPAddress: "10.13.14.15",
		Image:     "important-image:greatest",
		// No published port
	}))
	h.watchServices()
	h.si.processServiceUpdate(<-h.serviceUpdates)

	require.Len(t, h.allInstances(t), 1)
	svc, err := h.GetService("important-svc", store.QueryServiceOptions{WithInstances: true})
	require.Nil(t, err)
	require.Equal(t, "", svc.Instances[0].Address)
	require.Equal(t, 0, svc.Instances[0].Port)
	require.Equal(t, data.NOADDR, svc.Instances[0].State)
	h.stop(t)
}

func TestHostNetworking(t *testing.T) {
	h := setup(nil, "192.168.5.135", GLOBAL)

	h.AddService("blorp-svc", data.Service{
		InstancePort: 8080,
	})
	h.addGroup("blorp-svc", "image", "blorp-image")

	h.si.processContainerUpdate(resetContainers(container{
		NetworkMode: "host",
		ID:          "blorp-instance",
		IPAddress:   "",
		Image:       "blorp-image:tag",
	}))
	h.watchServices()
	h.si.processServiceUpdate(<-h.serviceUpdates)

	require.Len(t, h.allInstances(t), 1)
	svc, err := h.GetService("blorp-svc", store.QueryServiceOptions{WithInstances: true})
	require.Nil(t, err)
	require.Equal(t, h.si.hostIP, svc.Instances[0].Address)
	require.Equal(t, 8080, svc.Instances[0].Port)
	require.Equal(t, data.LIVE, svc.Instances[0].State)
	h.stop(t)
}

func TestOtherHostsEntries(t *testing.T) {
	st := inmem.NewInMemStore()
	h1 := setup(st, "192.168.11.34", LOCAL)
	h2 := setup(st, "192.168.11.5", LOCAL)

	h1.AddService("foo-svc", data.Service{})
	h1.addGroup("foo-svc", "image", "foo-image")
	h1.si.processContainerUpdate(resetContainers(container{
		ID:        "bar1",
		IPAddress: "192.168.34.1",
		Image:     "foo-image:version",
	}, container{
		ID:        "baz1",
		IPAddress: "192.168.34.2",
		Image:     "foo-image:version2",
	}))

	h2.si.processContainerUpdate(resetContainers(container{
		ID:        "bar2",
		IPAddress: "192.168.34.3",
		Image:     "foo-image:version",
	}, container{
		ID:        "baz2",
		IPAddress: "192.168.34.4",
		Image:     "foo-image:version2",
	}))

	// let si on the first host add its instances
	h1.watchServices()
	h1.si.processServiceUpdate(<-h1.serviceUpdates)
	require.Len(t, h1.allInstances(t), 2)

	// let si on the second host add its instances
	h2.watchServices()
	h2.si.processServiceUpdate(<-h2.serviceUpdates)
	require.Len(t, h1.allInstances(t), 4)

	// simulate an agent restart; in the meantime, a container has
	// stopped.
	h2.stop(t)
	h2 = setup(st, "192.168.11.5", LOCAL)
	h2.si.processContainerUpdate(resetContainers(container{
		ID:        "bar2",
		IPAddress: "192.168.34.3",
		Image:     "foo-image:version",
	}))
	h2.watchServices()
	h2.si.processServiceUpdate(<-h2.serviceUpdates)
	require.Len(t, h2.allInstances(t), 3)

	// test behaviour when the docker listener restarts:
	h2.si.processContainerUpdate(resetContainers())
	require.Len(t, h2.allInstances(t), 2)

	h1.stop(t)
	h2.stop(t)
}

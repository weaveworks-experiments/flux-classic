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

func setup(hostIP, netmode string) (*SyncInstances, store.Store) {
	st := inmem.NewInMemStore()
	if netmode == "" {
		netmode = LOCAL
	}
	return NewSyncInstances(Config{
		Store:   st,
		Network: netmode,
		HostIP:  hostIP,
	}), st
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

func TestSyncInstancesReconcile(t *testing.T) {
	si, st := setup("10.98.99.100", GLOBAL)
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

	si.ReadInServices()
	si.processContainerUpdate(resetContainers(container{
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

	insts := allInstances(st, t)
	require.Len(t, insts, 3)
	for _, inst := range insts {
		require.Equal(t, selectedAddress, inst.Address)
	}
}

func TestSyncInstancesEvents(t *testing.T) {
	si, st := setup("10.98.90.111", "")
	// starting condition
	require.Len(t, allInstances(st, t), 0)

	changes := make(chan data.ServiceChange, 1)

	st.WatchServices(nil, changes, daemon.NewErrorSink(),
		store.QueryServiceOptions{WithContainerRules: true})

	// no services defined
	si.processContainerUpdate(resetContainers(container{
		ID:        "foo",
		Image:     "foo-image:latest",
		IPAddress: "192.168.0.67",
	}))
	require.Len(t, allInstances(st, t), 0)

	st.AddService("foo-svc", data.Service{})
	si.processServiceChange(<-changes)
	addGroup(st, "foo-svc", "image", "foo-image")
	si.processServiceChange(<-changes)
	require.Len(t, allInstances(st, t), 1)

	addGroup(st, "foo-svc", "image", "not-foo-image")
	si.processServiceChange(<-changes)
	require.Len(t, allInstances(st, t), 0)

	si.processContainerUpdate(addContainer(container{
		ID:        "bar",
		IPAddress: "192.168.34.87",
		Image:     "not-foo-image:version",
	}))
	si.processContainerUpdate(addContainer(container{
		ID:        "baz",
		IPAddress: "192.168.34.99",
		Image:     "not-foo-image:version2",
	}))
	require.Len(t, allInstances(st, t), 2)

	si.processContainerUpdate(removeContainer("baz"))
	require.Len(t, allInstances(st, t), 1)

	st.RemoveService("foo-svc")
	si.processServiceChange(<-changes)
	require.Len(t, allInstances(st, t), 0)
}

func TestMappedPort(t *testing.T) {
	si, st := setup("11.98.99.98", LOCAL)

	st.AddService("blorp-svc", data.Service{
		InstancePort: 8080,
	})
	addGroup(st, "blorp-svc", "image", "blorp-image")

	si.processContainerUpdate(resetContainers(container{
		ID:        "blorp-instance",
		IPAddress: "10.13.14.15",
		Image:     "blorp-image:tag",
		Ports: map[string]string{
			"8080/tcp": "3456",
		},
	}))

	si.ReadInServices()

	require.Len(t, allInstances(st, t), 1)
	svc, err := st.GetService("blorp-svc", store.QueryServiceOptions{WithInstances: true})
	require.Nil(t, err)
	require.Equal(t, si.hostIP, svc.Instances[0].Address)
	require.Equal(t, 3456, svc.Instances[0].Port)
	require.Equal(t, data.LIVE, svc.Instances[0].State)
}

func TestMultihostNetworking(t *testing.T) {
	instAddress := "10.13.14.15"
	instPort := 8080

	si, st := setup("11.98.99.98", GLOBAL)

	st.AddService("blorp-svc", data.Service{
		InstancePort: instPort,
	})
	addGroup(st, "blorp-svc", "image", "blorp-image")

	si.processContainerUpdate(resetContainers(container{
		ID:        "blorp-instance",
		IPAddress: instAddress,
		Image:     "blorp-image:tag",
		Ports: map[string]string{
			"8080/tcp": "3456",
		},
	}))

	si.ReadInServices()

	require.Len(t, allInstances(st, t), 1)
	svc, err := st.GetService("blorp-svc", store.QueryServiceOptions{WithInstances: true})
	require.Nil(t, err)
	require.Equal(t, instAddress, svc.Instances[0].Address)
	require.Equal(t, instPort, svc.Instances[0].Port)
	require.Equal(t, data.LIVE, svc.Instances[0].State)
}

func TestNoAddress(t *testing.T) {
	si, st := setup("192.168.3.4", LOCAL)

	st.AddService("important-svc", data.Service{
		InstancePort: 80,
	})
	addGroup(st, "important-svc", "image", "important-image")

	si.processContainerUpdate(resetContainers(container{
		ID:        "oops-instance",
		IPAddress: "10.13.14.15",
		Image:     "important-image:greatest",
		// No published port
	}))

	si.ReadInServices()

	require.Len(t, allInstances(st, t), 1)
	svc, err := st.GetService("important-svc", store.QueryServiceOptions{WithInstances: true})
	require.Nil(t, err)
	require.Equal(t, "", svc.Instances[0].Address)
	require.Equal(t, 0, svc.Instances[0].Port)
	require.Equal(t, data.NOADDR, svc.Instances[0].State)
}

func TestHostNetworking(t *testing.T) {
	si, st := setup("192.168.5.135", GLOBAL)

	st.AddService("blorp-svc", data.Service{
		InstancePort: 8080,
	})
	addGroup(st, "blorp-svc", "image", "blorp-image")

	si.processContainerUpdate(resetContainers(container{
		NetworkMode: "host",
		ID:          "blorp-instance",
		IPAddress:   "",
		Image:       "blorp-image:tag",
	}))

	si.ReadInServices()

	require.Len(t, allInstances(st, t), 1)
	svc, err := st.GetService("blorp-svc", store.QueryServiceOptions{WithInstances: true})
	require.Nil(t, err)
	require.Equal(t, si.hostIP, svc.Instances[0].Address)
	require.Equal(t, 8080, svc.Instances[0].Port)
	require.Equal(t, data.LIVE, svc.Instances[0].State)
}

func TestOtherHostsEntries(t *testing.T) {
	si1, st := setup("192.168.11.34", LOCAL)
	si2 := NewSyncInstances(Config{
		Store:  st,
		HostIP: "192.168.11.5",
	})

	st.AddService("foo-svc", data.Service{})
	addGroup(st, "foo-svc", "image", "foo-image")
	si1.processContainerUpdate(resetContainers(container{
		ID:        "bar1",
		IPAddress: "192.168.34.1",
		Image:     "foo-image:version",
	}, container{
		ID:        "baz1",
		IPAddress: "192.168.34.2",
		Image:     "foo-image:version2",
	}))
	si2.processContainerUpdate(resetContainers(container{
		ID:        "bar2",
		IPAddress: "192.168.34.3",
		Image:     "foo-image:version",
	}, container{
		ID:        "baz2",
		IPAddress: "192.168.34.4",
		Image:     "foo-image:version2",
	}))
	// let si on the first host add its instances
	si1.ReadInServices()

	require.Len(t, allInstances(st, t), 2)

	// let si on the second host add its instances
	si2.ReadInServices()

	require.Len(t, allInstances(st, t), 4)

	// simulate an agent restart; in the meantime, a container has
	// stopped.
	si2 = NewSyncInstances(Config{
		Store:   st,
		Network: LOCAL,
		HostIP:  "192.168.11.5",
	})
	si2.processContainerUpdate(resetContainers(container{
		ID:        "bar2",
		IPAddress: "192.168.34.3",
		Image:     "foo-image:version",
	}))
	si2.ReadInServices()
	require.Len(t, allInstances(st, t), 3)

	// test behaviour when the docker listener restarts:
	si2.processContainerUpdate(resetContainers())
	require.Len(t, allInstances(st, t), 2)
}

package agent

import (
	"net"
	"reflect"
	"strings"
	"testing"

	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/netutil"
	"github.com/weaveworks/flux/common/store"
	"github.com/weaveworks/flux/common/store/inmem"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/stretchr/testify/require"
)

func makePump(chanPtr interface{}) func() {
	chanPtrT := reflect.TypeOf(chanPtr)
	if chanPtrT.Kind() != reflect.Ptr {
		panic("Non-pointer passed to makePump")
	}

	chanT := chanPtrT.Elem()
	if chanT.Kind() != reflect.Chan ||
		chanT.ChanDir() != reflect.RecvDir {
		panic("Pointer to a non-channel (or non-receive-channel) passed to makePump")
	}

	chanV := reflect.ValueOf(chanPtr).Elem()
	origChanV := reflect.ValueOf(chanV.Interface())
	substChanV := reflect.MakeChan(reflect.ChanOf(reflect.BothDir,
		chanT.Elem()), 0)
	chanV.Set(substChanV)
	return func() {
		val, ok := origChanV.Recv()
		if !ok {
			panic("Channel closed")
		}
		substChanV.Send(val)
	}
}

type containerInfo struct {
	ID          string
	IPAddress   string
	Image       string
	Labels      map[string]string
	Env         map[string]string
	Ports       map[string]string
	NetworkMode string
}

func makeContainersMap(cs []containerInfo) map[string]*docker.Container {
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
	hostIP net.IP
	errs   daemon.ErrorSink
	store.Store
	containerUpdates   chan ContainerUpdate
	instanceUpdates    chan LocalInstanceUpdate
	syncInstances      daemon.Component
	watchingServices   daemon.Component
	serviceUpdatesPump func()
}

func setup(hostIP, netmode string) harness {
	serviceUpdates := make(chan store.ServiceUpdate)
	var serviceUpdatesRecv <-chan store.ServiceUpdate = serviceUpdates

	h := harness{
		hostIP:             net.ParseIP(hostIP),
		errs:               daemon.NewErrorSink(),
		Store:              inmem.NewInMem().Store("test session"),
		containerUpdates:   make(chan ContainerUpdate),
		instanceUpdates:    make(chan LocalInstanceUpdate, 10),
		serviceUpdatesPump: makePump(&serviceUpdatesRecv),
	}

	h.syncInstances = syncInstancesConfig{
		network: netmode,
		hostIP:  h.hostIP,

		containerUpdates:      h.containerUpdates,
		containerUpdatesReset: make(chan struct{}, 100),
		serviceUpdates:        serviceUpdatesRecv,
		serviceUpdatesReset:   make(chan struct{}, 100),
		localInstanceUpdates:  h.instanceUpdates,
	}.StartFunc()(h.errs)

	h.watchingServices = store.WatchServicesStartFunc(h.Store,
		store.QueryServiceOptions{WithContainerRules: true},
		serviceUpdates)(h.errs)

	return h
}

func (h *harness) stop(t *testing.T) {
	h.syncInstances.Stop()
	h.watchingServices.Stop()
	require.Empty(t, h.errs)
}

func (h *harness) resetContainers(cs ...containerInfo) {
	h.containerUpdates <- ContainerUpdate{
		Containers: makeContainersMap(cs),
		Reset:      true,
	}
}

func (h *harness) addContainers(cs ...containerInfo) {
	h.containerUpdates <- ContainerUpdate{
		Containers: makeContainersMap(cs),
	}
}

func (h *harness) removeContainer(id string) {
	h.containerUpdates <- ContainerUpdate{
		Containers: map[string]*docker.Container{id: nil},
	}
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
		store.ContainerRule{Selector: sel})
}

func (iu LocalInstanceUpdate) get(svc, inst string) *store.Instance {
	return iu.LocalInstances[InstanceKey{Service: svc, Instance: inst}]
}

func TestSyncInstancesReconcile(t *testing.T) {
	h := setup("10.98.99.100", GLOBAL)
	h.AddService("foo-svc", store.Service{
		InstancePort: 80,
	})
	h.addGroup("foo-svc", "tag", "bobbins", "image", "foo-image")
	h.AddService("bar-svc", store.Service{
		InstancePort: 80,
	})
	h.addGroup("bar-svc", "flux/foo-label", "blorp")
	h.AddService("boo-svc", store.Service{
		InstancePort: 80,
	})
	h.addGroup("boo-svc", "env.SERVICE_NAME", "boo")

	selectedAddress := net.ParseIP("192.168.45.67")

	h.resetContainers(containerInfo{
		ID:        "selected",
		IPAddress: selectedAddress.String(),
		Image:     "foo-image:bobbins",
		Labels:    map[string]string{"flux/foo-label": "blorp"},
		Env:       map[string]string{"SERVICE_NAME": "boo"},
	}, containerInfo{
		ID:        "not",
		IPAddress: "111.111.111.111",
		Image:     "foo-image:not-bobbins",
		Labels:    map[string]string{"flux/foo-label": "something-else"},
		Env:       map[string]string{"SERVICE_NAME": "literally anything"},
	})

	h.serviceUpdatesPump()
	iu := <-h.instanceUpdates
	require.True(t, iu.Reset)
	require.Len(t, iu.LocalInstances, 3)
	require.Equal(t, selectedAddress, iu.get("foo-svc", "selected").Address.IP)
	require.Equal(t, selectedAddress, iu.get("bar-svc", "selected").Address.IP)
	require.Equal(t, selectedAddress, iu.get("boo-svc", "selected").Address.IP)
	h.stop(t)
}

func TestSyncInstancesEvents(t *testing.T) {
	h := setup("10.98.90.111", LOCAL)

	// no services defined
	h.resetContainers(containerInfo{
		ID:        "foo",
		Image:     "foo-image:latest",
		IPAddress: "192.168.0.67",
	})
	h.serviceUpdatesPump()
	iu := <-h.instanceUpdates
	require.True(t, iu.Reset)
	require.Len(t, iu.LocalInstances, 0)

	h.AddService("foo-svc", store.Service{})
	h.serviceUpdatesPump()
	h.addGroup("foo-svc", "image", "foo-image")
	h.serviceUpdatesPump()
	iu = <-h.instanceUpdates
	require.False(t, iu.Reset)
	require.Len(t, iu.LocalInstances, 1)

	h.addGroup("foo-svc", "image", "not-foo-image")
	h.serviceUpdatesPump()
	iu = <-h.instanceUpdates
	require.False(t, iu.Reset)
	require.Len(t, iu.LocalInstances, 1)

	h.addContainers(containerInfo{
		ID:        "bar",
		IPAddress: "192.168.34.87",
		Image:     "not-foo-image:version",
	}, containerInfo{
		ID:        "baz",
		IPAddress: "192.168.34.99",
		Image:     "not-foo-image:version2",
	})
	iu = <-h.instanceUpdates
	require.False(t, iu.Reset)
	require.Len(t, iu.LocalInstances, 2)

	h.removeContainer("baz")
	iu = <-h.instanceUpdates
	require.False(t, iu.Reset)
	require.Len(t, iu.LocalInstances, 1)

	h.RemoveService("foo-svc")
	h.serviceUpdatesPump()
	iu = <-h.instanceUpdates
	require.False(t, iu.Reset)
	require.Equal(t, map[InstanceKey]*store.Instance{
		InstanceKey{Service: "foo-svc", Instance: "bar"}: nil,
	}, iu.LocalInstances)

	h.stop(t)
}

func TestMappedPort(t *testing.T) {
	h := setup("11.98.99.98", LOCAL)
	h.AddService("blorp-svc", store.Service{
		InstancePort: 8080,
	})
	h.addGroup("blorp-svc", "image", "blorp-image")

	h.resetContainers(containerInfo{
		ID:        "blorp-instance",
		IPAddress: "10.13.14.15",
		Image:     "blorp-image:tag",
		Ports: map[string]string{
			"8080/tcp": "3456",
		},
	})

	h.serviceUpdatesPump()
	iu := <-h.instanceUpdates
	require.True(t, iu.Reset)
	require.Len(t, iu.LocalInstances, 1)
	require.Equal(t, &netutil.IPPort{h.hostIP, 3456}, iu.get("blorp-svc", "blorp-instance").Address)
	h.stop(t)
}

func TestMultihostNetworking(t *testing.T) {
	instAddress := net.ParseIP("10.13.14.15")
	instPort := 8080

	h := setup("11.98.99.98", GLOBAL)

	h.AddService("blorp-svc", store.Service{
		InstancePort: instPort,
	})
	h.addGroup("blorp-svc", "image", "blorp-image")

	h.resetContainers(containerInfo{
		ID:        "blorp-instance",
		IPAddress: instAddress.String(),
		Image:     "blorp-image:tag",
		Ports: map[string]string{
			"8080/tcp": "3456",
		},
	})

	h.serviceUpdatesPump()
	iu := <-h.instanceUpdates
	require.True(t, iu.Reset)
	require.Len(t, iu.LocalInstances, 1)
	require.Equal(t, &netutil.IPPort{instAddress, instPort}, iu.get("blorp-svc", "blorp-instance").Address)
	h.stop(t)
}

func TestNoAddress(t *testing.T) {
	h := setup("192.168.3.4", LOCAL)

	h.AddService("important-svc", store.Service{
		InstancePort: 80,
	})
	h.addGroup("important-svc", "image", "important-image")

	h.resetContainers(containerInfo{
		ID:        "oops-instance",
		IPAddress: "10.13.14.15",
		Image:     "important-image:greatest",
		// No published port
	})

	h.serviceUpdatesPump()
	iu := <-h.instanceUpdates
	require.True(t, iu.Reset)
	require.Len(t, iu.LocalInstances, 1)
	require.Nil(t, iu.get("important-svc", "oops-instance").Address)
	h.stop(t)
}

func TestHostNetworking(t *testing.T) {
	h := setup("192.168.5.135", GLOBAL)

	h.AddService("blorp-svc", store.Service{
		InstancePort: 8080,
	})
	h.addGroup("blorp-svc", "image", "blorp-image")

	h.resetContainers(containerInfo{
		NetworkMode: "host",
		ID:          "blorp-instance",
		IPAddress:   "",
		Image:       "blorp-image:tag",
	})

	h.serviceUpdatesPump()
	iu := <-h.instanceUpdates
	require.True(t, iu.Reset)
	require.Len(t, iu.LocalInstances, 1)
	require.Equal(t, &netutil.IPPort{h.hostIP, 8080}, iu.get("blorp-svc", "blorp-instance").Address)
	h.stop(t)
}

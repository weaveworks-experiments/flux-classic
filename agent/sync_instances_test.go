package agent

import (
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/netutil"
	"github.com/weaveworks/flux/common/store"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/stretchr/testify/require"
)

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
	hostIP                net.IP
	errs                  daemon.ErrorSink
	serviceUpdates        chan store.ServiceUpdate
	serviceUpdatesReset   chan struct{}
	containerUpdates      chan ContainerUpdate
	containerUpdatesReset chan struct{}
	instanceUpdates       chan InstanceUpdate
	instanceUpdatesReset  chan struct{}
	syncInstances         daemon.Component
}

func setup(hostIP, netmode string) harness {
	h := harness{
		hostIP:                net.ParseIP(hostIP),
		errs:                  daemon.NewErrorSink(),
		serviceUpdates:        make(chan store.ServiceUpdate),
		containerUpdatesReset: make(chan struct{}, 10),
		containerUpdates:      make(chan ContainerUpdate),
		serviceUpdatesReset:   make(chan struct{}, 10),
		instanceUpdates:       make(chan InstanceUpdate),
		instanceUpdatesReset:  make(chan struct{}),
	}

	h.syncInstances = syncInstancesConfig{
		network: netmode,
		hostIP:  h.hostIP,

		containerUpdates:      h.containerUpdates,
		containerUpdatesReset: h.containerUpdatesReset,
		serviceUpdates:        h.serviceUpdates,
		serviceUpdatesReset:   h.serviceUpdatesReset,
		instanceUpdates:       h.instanceUpdates,
		instanceUpdatesReset:  h.instanceUpdatesReset,
	}.StartFunc()(h.errs)

	return h
}

func (h *harness) stop(t *testing.T) {
	h.syncInstances.Stop()
	require.Empty(t, h.errs)
}

func (h *harness) addContainers(reset bool, cs ...containerInfo) {
	h.containerUpdates <- ContainerUpdate{
		Containers: makeContainersMap(cs),
		Reset:      reset,
	}
}

func (h *harness) removeContainer(id string) {
	h.containerUpdates <- ContainerUpdate{
		Containers: map[string]*docker.Container{id: nil},
	}
}

func makeRule(port int, labels ...string) store.ContainerRule {
	if len(labels)%2 != 0 {
		panic("Expected key value ... as arguments")
	}
	sel := make(map[string]string)
	for i := 0; i < len(labels); i += 2 {
		sel[labels[i]] = labels[i+1]
	}
	return store.ContainerRule{sel, port}
}

func namedRule(name string, port int, labels ...string) map[string]store.ContainerRule {
	return map[string]store.ContainerRule{
		name: makeRule(port, labels...),
	}
}

func rule(labels ...string) map[string]store.ContainerRule {
	return namedRule(GROUP, 0, labels...)
}

func serviceUpdate(reset bool, name string, svc store.ServiceInfo) store.ServiceUpdate {
	return store.ServiceUpdate{
		Services: map[string]*store.ServiceInfo{name: &svc},
		Reset:    reset,
	}
}

func (iu InstanceUpdate) get(svc, inst string) *store.Instance {
	return iu.Instances[InstanceKey{Service: svc, Instance: inst}]
}

func TestSyncInstancesReconcile(t *testing.T) {
	h := setup("10.98.99.100", GLOBAL)

	h.serviceUpdates <- serviceUpdate(true, "foo-svc", store.ServiceInfo{
		Service:        store.Service{InstancePort: 80},
		ContainerRules: rule("tag", "bobbins", "image", "foo-image"),
	})
	h.serviceUpdates <- serviceUpdate(false, "bar-svc", store.ServiceInfo{
		Service:        store.Service{InstancePort: 80},
		ContainerRules: rule("flux/foo-label", "blorp"),
	})
	h.serviceUpdates <- serviceUpdate(false, "boo-svc", store.ServiceInfo{
		Service:        store.Service{InstancePort: 80},
		ContainerRules: rule("env.SERVICE_NAME", "boo"),
	})

	selectedAddress := net.ParseIP("192.168.45.67")

	h.addContainers(true, containerInfo{
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

	iu := <-h.instanceUpdates
	require.True(t, iu.Reset)
	require.Len(t, iu.Instances, 3)
	require.Equal(t, selectedAddress, iu.get("foo-svc", "selected").Address.IP)
	require.Equal(t, selectedAddress, iu.get("bar-svc", "selected").Address.IP)
	require.Equal(t, selectedAddress, iu.get("boo-svc", "selected").Address.IP)
	h.stop(t)
}

func TestSyncInstancesEvents(t *testing.T) {
	h := setup("10.98.90.111", LOCAL)

	// no services defined
	h.addContainers(true, containerInfo{
		ID:        "foo",
		Image:     "foo-image:latest",
		IPAddress: "192.168.0.67",
	})
	h.serviceUpdates <- store.ServiceUpdate{Reset: true}
	iu := <-h.instanceUpdates
	require.True(t, iu.Reset)
	require.Len(t, iu.Instances, 0)

	// Add a service with a matching rule
	h.serviceUpdates <- serviceUpdate(false, "foo-svc", store.ServiceInfo{
		Service:        store.Service{InstancePort: 80},
		ContainerRules: rule("image", "foo-image"),
	})
	iu = <-h.instanceUpdates
	require.False(t, iu.Reset)
	require.Len(t, iu.Instances, 1)

	// Replace with a non-matching rule
	h.serviceUpdates <- serviceUpdate(false, "foo-svc", store.ServiceInfo{
		Service:        store.Service{InstancePort: 80},
		ContainerRules: rule("image", "not-foo-image"),
	})
	iu = <-h.instanceUpdates
	require.False(t, iu.Reset)
	require.Equal(t, map[InstanceKey]*store.Instance{
		InstanceKey{Service: "foo-svc", Instance: "foo"}: nil,
	}, iu.Instances)

	// Add some matching containers
	h.addContainers(false, containerInfo{
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
	require.Len(t, iu.Instances, 2)

	// Remove a container
	h.removeContainer("baz")
	iu = <-h.instanceUpdates
	require.False(t, iu.Reset)
	require.Len(t, iu.Instances, 1)

	// Remove a service
	h.serviceUpdates <- store.ServiceUpdate{
		Services: map[string]*store.ServiceInfo{
			"foo-svc": nil,
		},
	}
	iu = <-h.instanceUpdates
	require.False(t, iu.Reset)
	require.Equal(t, map[InstanceKey]*store.Instance{
		InstanceKey{Service: "foo-svc", Instance: "bar"}: nil,
	}, iu.Instances)

	h.stop(t)
}

func testMappedPort(t *testing.T, svc store.ServiceInfo, usedPort int) {
	h := setup("10.98.90.111", LOCAL)

	h.serviceUpdates <- serviceUpdate(true, "blorp-svc", svc)

	h.addContainers(true, containerInfo{
		ID:        "blorp-instance",
		IPAddress: "10.13.14.15",
		Image:     "blorp-image:tag",
		Ports: map[string]string{
			fmt.Sprintf("%d/tcp", usedPort): "3456",
		},
	})

	iu := <-h.instanceUpdates
	require.True(t, iu.Reset)
	require.Len(t, iu.Instances, 1)
	require.Equal(t, &netutil.IPPort{h.hostIP, 3456}, iu.get("blorp-svc", "blorp-instance").Address)
	h.stop(t)
}

func TestServiceMappedPort(t *testing.T) {
	testMappedPort(t, store.ServiceInfo{
		Service:        store.Service{InstancePort: 8080},
		ContainerRules: rule("image", "blorp-image"),
	}, 8080)
}

func TestRuleMappedPort(t *testing.T) {
	testMappedPort(t, store.ServiceInfo{
		Service:        store.Service{InstancePort: 0},
		ContainerRules: namedRule(GROUP, 1234, "image", "blorp-image"),
	}, 1234)
}

func TestMultihostNetworking(t *testing.T) {
	instAddress := net.ParseIP("10.13.14.15")
	instPort := 8080

	h := setup("11.98.99.98", GLOBAL)

	h.serviceUpdates <- serviceUpdate(true, "blorp-svc", store.ServiceInfo{
		Service:        store.Service{InstancePort: instPort},
		ContainerRules: rule("image", "blorp-image"),
	})

	h.addContainers(true, containerInfo{
		ID:        "blorp-instance",
		IPAddress: instAddress.String(),
		Image:     "blorp-image:tag",
		Ports: map[string]string{
			"8080/tcp": "3456",
		},
	})

	iu := <-h.instanceUpdates
	require.True(t, iu.Reset)
	require.Len(t, iu.Instances, 1)
	require.Equal(t, &netutil.IPPort{instAddress, instPort}, iu.get("blorp-svc", "blorp-instance").Address)
	h.stop(t)
}

func TestNoAddress(t *testing.T) {
	h := setup("192.168.3.4", LOCAL)

	h.serviceUpdates <- serviceUpdate(true, "important-svc", store.ServiceInfo{
		Service:        store.Service{InstancePort: 80},
		ContainerRules: rule("image", "important-image"),
	})

	h.addContainers(true, containerInfo{
		ID:        "oops-instance",
		IPAddress: "10.13.14.15",
		Image:     "important-image:greatest",
		// No published port
	})

	iu := <-h.instanceUpdates
	require.True(t, iu.Reset)
	require.Len(t, iu.Instances, 1)
	require.Nil(t, iu.get("important-svc", "oops-instance").Address)
	h.stop(t)
}

func TestHostNetworking(t *testing.T) {
	h := setup("192.168.5.135", GLOBAL)

	h.serviceUpdates <- serviceUpdate(true, "blorp-svc", store.ServiceInfo{
		Service:        store.Service{InstancePort: 8080},
		ContainerRules: rule("image", "blorp-image"),
	})

	h.addContainers(true, containerInfo{
		NetworkMode: "host",
		ID:          "blorp-instance",
		IPAddress:   "",
		Image:       "blorp-image:tag",
	})

	iu := <-h.instanceUpdates
	require.True(t, iu.Reset)
	require.Len(t, iu.Instances, 1)
	require.Equal(t, &netutil.IPPort{h.hostIP, 8080}, iu.get("blorp-svc", "blorp-instance").Address)
	h.stop(t)
}

func TestSyncInstancesResets(t *testing.T) {
	h := setup("10.98.90.111", LOCAL)

	// reset signals should have been sent out initially
	<-h.serviceUpdatesReset
	<-h.containerUpdatesReset

	// Send some initial non-reset events before the resets.
	// These will get ignored until syncInstances has got the
	// complete state.
	sendService := func(reset bool, name string) {
		h.serviceUpdates <- serviceUpdate(reset, name, store.ServiceInfo{
			Service:        store.Service{InstancePort: 8080},
			ContainerRules: rule("image", "blorp-image"),
		})
	}
	sendService(false, "blorp-svc")

	sendContainer := func(reset bool, name string) {
		h.addContainers(reset, containerInfo{
			ID:        name,
			IPAddress: "10.13.14.15",
			Image:     "blorp-image:tag",
			Ports: map[string]string{
				"8080/tcp": "3456",
			},
		})
	}
	sendContainer(false, "foo")

	// Populate things
	sendService(true, "blorp-svc")
	sendContainer(true, "foo")

	iu := <-h.instanceUpdates
	require.True(t, iu.Reset)
	require.Len(t, iu.Instances, 1)

	// Simulate a docker restart
	sendContainer(true, "bar")

	iu = <-h.instanceUpdates
	require.True(t, iu.Reset)
	require.Len(t, iu.Instances, 1)

	// Simulate an etcd restart
	sendService(true, "blorp-svc-2")

	iu = <-h.instanceUpdates
	require.True(t, iu.Reset)
	require.Len(t, iu.Instances, 1)

	// request another reset from downstream
	require.Len(t, h.serviceUpdatesReset, 0)
	require.Len(t, h.containerUpdatesReset, 0)
	h.instanceUpdatesReset <- struct{}{}
	<-h.serviceUpdatesReset
	<-h.containerUpdatesReset

	h.stop(t)
}

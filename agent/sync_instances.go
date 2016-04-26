package agent

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/netutil"
	"github.com/weaveworks/flux/common/store"

	log "github.com/Sirupsen/logrus"
	docker "github.com/fsouza/go-dockerclient"
)

type InstanceKey struct {
	Service, Instance string
}

type InstanceUpdate struct {
	Instances map[InstanceKey]*store.Instance
	Reset     bool
}

type syncInstancesConfig struct {
	hostIP  net.IP
	network string

	containerUpdates      <-chan ContainerUpdate
	containerUpdatesReset chan<- struct{}
	serviceUpdates        <-chan store.ServiceUpdate
	serviceUpdatesReset   chan<- struct{}
	instanceUpdates       chan<- InstanceUpdate
	instanceUpdatesReset  <-chan struct{}
}

type syncInstances struct {
	syncInstancesConfig
	errs       daemon.ErrorSink
	services   map[string]service
	containers map[string]container

	// The InstanceUpdate update being currently accumulated
	update InstanceUpdate
}

type service struct {
	name string
	*store.ServiceInfo

	// map from instance names (= container ids)
	instances map[string]*store.Instance
}

type container struct {
	*docker.Container

	// map from services names with instances for this container
	instances map[string]struct{}
}

func (conf syncInstancesConfig) StartFunc() daemon.StartFunc {
	return daemon.SimpleComponent(conf.start)
}

func (conf syncInstancesConfig) start(stop <-chan struct{}, errs daemon.ErrorSink) {
	si := syncInstances{
		syncInstancesConfig: conf,
		errs:                errs,
	}

	si.containerUpdatesReset <- struct{}{}
	si.serviceUpdatesReset <- struct{}{}

	for {
		// Clear the current update
		si.update.Instances = make(map[InstanceKey]*store.Instance)
		si.update.Reset = false

		select {
		case update := <-si.containerUpdates:
			si.processContainerUpdate(update)

		case update := <-si.serviceUpdates:
			si.processServiceUpdate(update)

		case <-si.instanceUpdatesReset:
			// Drop state, ask for resets from our sources
			si.services = nil
			si.containers = nil
			si.containerUpdatesReset <- struct{}{}
			si.serviceUpdatesReset <- struct{}{}

		case <-stop:
			return
		}

		if len(si.update.Instances) > 0 || si.update.Reset {
			si.instanceUpdates <- si.update
			si.update.Instances = nil
		}
	}
}

func (si *syncInstances) processContainerUpdate(update ContainerUpdate) {
	if update.Reset {
		si.containers = make(map[string]container)
		si.clearInstances()
		// Only send a reset when we have full information
		si.update.Reset = (si.services != nil)
	}

	for id, cont := range update.Containers {
		if cont != nil {
			si.addContainer(cont)
		} else {
			si.removeContainer(id)
		}
	}
}

func (si *syncInstances) addContainer(cont *docker.Container) {
	if si.containers == nil {
		return
	}

	if _, found := si.containers[cont.ID]; found {
		return
	}

	c := container{Container: cont, instances: make(map[string]struct{})}
	si.containers[cont.ID] = c

	for svcName, svc := range si.services {
		log.Infof(`Evaluating container '%s' against service '%s'`, cont.ID, svcName)
		si.addInstances(svc, c)
	}
}

func (si *syncInstances) removeContainer(id string) {
	if cont, found := si.containers[id]; found {
		delete(si.containers, id)

		if si.services != nil {
			for svcName := range cont.instances {
				delete(si.services[svcName].instances, id)
				si.updateInstance(svcName, id, nil)
			}
		}
	}
}

func (si *syncInstances) addInstances(svc service, cont container) {
	for _, rule := range svc.ContainerRules {
		inst := si.extractInstance(cont.Container, svc.ServiceInfo,
			rule)
		if inst != nil {
			svc.instances[cont.ID] = inst
			cont.instances[svc.name] = struct{}{}
			si.updateInstance(svc.name, cont.ID, inst)
		}
	}
}

func (si *syncInstances) updateInstance(svcName, instName string, inst *store.Instance) {
	key := InstanceKey{Service: svcName, Instance: instName}
	si.update.Instances[key] = inst
}

func (si *syncInstances) processServiceUpdate(update store.ServiceUpdate) {
	if update.Reset {
		si.services = make(map[string]service)
		si.clearInstances()
		// Only send a reset when we have full information
		si.update.Reset = (si.containers != nil)
	}

	for svcName, svcInfo := range update.Services {
		if svcInfo != nil {
			si.updateService(svcName, svcInfo)
		} else {
			si.removeService(svcName)
		}
	}
}

func (si *syncInstances) updateService(svcName string, svcInfo *store.ServiceInfo) {
	if si.services == nil {
		return
	}

	svc := service{
		name:        svcName,
		ServiceInfo: svcInfo,
		instances:   make(map[string]*store.Instance),
	}
	old := si.services[svcName]
	si.services[svcName] = svc

	for _, cont := range si.containers {
		si.addInstances(svc, cont)
	}

	// See if any instances should go away
	for instName := range old.instances {
		if svc.instances[instName] == nil {
			si.updateInstance(svcName, instName, nil)
		}
	}
}

func (si *syncInstances) removeService(svcName string) {
	if svc, found := si.services[svcName]; found {
		delete(si.services, svcName)

		if si.containers != nil {
			for id := range svc.instances {
				delete(si.containers[id].instances, svcName)
				si.updateInstance(svcName, id, nil)
			}
		}
	}
}

func (si *syncInstances) clearInstances() {
	for _, svc := range si.services {
		for k := range svc.instances {
			delete(svc.instances, k)
		}
	}

	for _, cont := range si.containers {
		for k := range cont.instances {
			delete(cont.instances, k)
		}
	}
}

func (si *syncInstances) extractInstance(container *docker.Container, svc *store.ServiceInfo, rule store.ContainerRule) *store.Instance {
	if !rule.Includes(containerLabels{container}) {
		return nil
	}

	addr := si.extractAddress(container, svc)
	if addr == nil {
		log.Infof(`Cannot extract address for instance, from container '%s'`, container.ID)
	}

	labels := map[string]string{
		"tag":   imageTag(container.Config.Image),
		"image": imageName(container.Config.Image),
	}

	for k, v := range container.Config.Labels {
		labels[k] = v
	}

	for _, v := range container.Config.Env {
		kv := strings.SplitN(v, "=", 2)
		labels["env."+kv[0]] = kv[1]
	}

	return &store.Instance{
		Address: addr,
		Labels:  labels,
		Host:    store.Host{IP: si.hostIP},
	}
}

type containerLabels struct{ *docker.Container }

func (container containerLabels) Label(label string) string {
	switch {
	case label == "image":
		return imageName(container.Config.Image)
	case label == "tag":
		return imageTag(container.Config.Image)
	case len(label) > 4 && label[:4] == "env.":
		return envValue(container.Config.Env, label[4:])
	default:
		return container.Config.Labels[label]
	}
}

/*
Extract an address from a container, according to what we've been told
about the service.

There are two special cases:
 - if the service has no instance port, we have no chance of getting an
address, so just let the container be considered unaddressable;
 - if the container has been run with `--net=host`; this means the
container is using the host's networking stack, so we should use the
host IP address.

*/
func (si *syncInstances) extractAddress(container *docker.Container, svc *store.ServiceInfo) *netutil.IPPort {
	if svc.InstancePort == 0 {
		return nil
	}
	if container.HostConfig.NetworkMode == "host" {
		return &netutil.IPPort{si.hostIP, svc.InstancePort}
	}
	switch si.network {
	case LOCAL:
		return si.mappedPortAddress(container, svc.InstancePort)
	case GLOBAL:
		return si.fixedPortAddress(container, svc.InstancePort)
	}
	return nil
}

/*
Extract a "mapped port" address. This mode assumes the balancer is
connecting to containers via a port "mapped" (NATed) by
Docker. Therefore it looks for the port mentioned in the list of
published ports, and finds the host port it has been mapped to. The IP
address is that given as the host's IP address.
*/
func (si *syncInstances) mappedPortAddress(container *docker.Container, port int) *netutil.IPPort {
	p := docker.Port(fmt.Sprintf("%d/tcp", port))
	if bindings, found := container.NetworkSettings.Ports[p]; found {
		for _, binding := range bindings {
			switch binding.HostIP {
			case "", "0.0.0.0":
				// matches
			default:
				ip := net.ParseIP(binding.HostIP)
				if ip == nil || !ip.Equal(si.hostIP) {
					continue
				}
			}

			mappedToPort, err := strconv.Atoi(binding.HostPort)
			if err != nil {
				return nil
			}

			return &netutil.IPPort{si.hostIP, mappedToPort}
		}
	}

	return nil
}

/*
Extract a "fixed port" address. This mode assumes that the balancer
will be able to connect to the container, potentially across hosts,
using the address Docker has assigned it.
*/
func (si *syncInstances) fixedPortAddress(container *docker.Container, port int) *netutil.IPPort {
	ip := net.ParseIP(container.NetworkSettings.IPAddress)
	if ip == nil {
		return nil
	}

	return &netutil.IPPort{ip, port}
}

func envValue(env []string, key string) string {
	for _, entry := range env {
		keyval := strings.Split(entry, "=")
		if keyval[0] == key {
			return keyval[1]
		}
	}
	return ""
}

func imageTag(image string) string {
	colon := strings.LastIndex(image, ":")
	if colon == -1 {
		return "latest"
	}
	return image[colon+1:]
}

func imageName(image string) string {
	colon := strings.LastIndex(image, ":")
	if colon == -1 {
		return image
	}
	return image[:colon]
}

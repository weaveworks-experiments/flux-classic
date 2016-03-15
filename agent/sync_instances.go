package agent

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/data"
	"github.com/weaveworks/flux/common/store"

	log "github.com/Sirupsen/logrus"
	docker "github.com/fsouza/go-dockerclient"
)

type instanceSet map[string]struct{}

const (
	GLOBAL = "global"
	LOCAL  = "local"
)

func IsValidNetworkMode(mode string) bool {
	return mode == GLOBAL || mode == LOCAL
}

type service struct {
	*store.ServiceInfo
	localInstances instanceSet
}

func (svc *service) includes(instanceName string) bool {
	_, ok := svc.localInstances[instanceName]
	return ok
}

type SyncInstancesConfig struct {
	HostIP  string
	Network string
	Store   store.Store

	ContainerUpdates      <-chan ContainerUpdate
	ContainerUpdatesReset chan<- struct{}
	ServiceUpdates        <-chan store.ServiceUpdate
	ServiceUpdatesReset   chan<- struct{}
}

type syncInstances struct {
	SyncInstancesConfig
	daemon.ErrorSink
	services   map[string]*service
	containers map[string]*docker.Container
}

func (conf SyncInstancesConfig) StartFunc() daemon.StartFunc {
	return daemon.SimpleComponent(func(stop <-chan struct{}, errs daemon.ErrorSink) {
		si := syncInstances{
			SyncInstancesConfig: conf,
			ErrorSink:           errs,
		}

		si.ContainerUpdatesReset <- struct{}{}
		si.ServiceUpdatesReset <- struct{}{}

		for {
			select {
			case update := <-si.ContainerUpdates:
				si.processContainerUpdate(update)

			case update := <-si.ServiceUpdates:
				si.processServiceUpdate(update)

			case <-stop:
				return
			}
		}
	})
}

func (si *syncInstances) processContainerUpdate(update ContainerUpdate) {
	if update.Reset {
		si.containers = update.Containers
		for _, svc := range si.services {
			si.Post(si.syncInstances(svc))
		}

		return
	}

	for id, cont := range update.Containers {
		if cont != nil {
			si.containers[id] = cont
			si.Post(si.addContainer(cont))
		} else if cont := si.containers[id]; cont != nil {
			delete(si.containers, id)
			si.Post(si.removeContainer(cont))
		}
	}
}

func (si *syncInstances) addContainer(container *docker.Container) error {
	for _, service := range si.services {
		log.Infof(`Evaluating container '%s' against service '%s'`, container.ID, service.Name)
		if err := si.evaluate(container, service); err != nil {
			return err
		}
	}
	return nil
}

func (si *syncInstances) removeContainer(container *docker.Container) error {
	instName := instanceNameFor(container)
	for serviceName, svc := range si.services {
		if svc.includes(instName) {
			err := si.Store.RemoveInstance(serviceName, instName)
			if err != nil {
				return err
			}
			log.Infof("Deregistered service '%s' instance '%.12s'", serviceName, instName)
			delete(svc.localInstances, instName)
		}
	}
	return nil
}

func (si *syncInstances) processServiceUpdate(update store.ServiceUpdate) {
	if update.Reset {
		si.services = make(map[string]*service)
	}

	for name, svcInfo := range update.Services {
		if svcInfo != nil {
			svc := si.redefineService(svcInfo)
			si.Post(si.syncInstances(svc))
		} else if svc := si.containers[name]; svc != nil {
			delete(si.services, name)
		}
	}
}

// The service has been changed; re-evaluate which containers belong,
// and which don't. Assume we have a correct list of containers.
func (si *syncInstances) redefineService(svcInfo *store.ServiceInfo) *service {
	svc, found := si.services[svcInfo.Name]
	if !found {
		svc = &service{}
		si.services[svcInfo.Name] = svc
	}
	svc.ServiceInfo = svcInfo
	return svc
}

func (si *syncInstances) syncInstances(svc *service) error {
	if si.containers == nil {
		// Defer syncing instances until we learn about containers
		return nil
	}

	svc.localInstances = make(instanceSet)
	for _, container := range si.containers {
		if err := si.evaluate(container, svc); err != nil {
			return err
		}
	}

	// remove any instances for this service that do not match
	storeSvc, err := si.Store.GetService(svc.Name, store.QueryServiceOptions{WithInstances: true})
	if err != nil {
		return err
	}

	for _, inst := range storeSvc.Instances {
		if !svc.includes(inst.Name) && si.owns(inst.Instance) {
			if err := si.Store.RemoveInstance(svc.Name, inst.Name); err != nil {
				return err
			}
		}
	}

	return nil
}

func (si *syncInstances) owns(inst data.Instance) bool {
	return si.HostIP == inst.Host
}

func (si *syncInstances) evaluate(container *docker.Container, svc *service) error {
	for _, spec := range svc.ContainerRules {
		if instance, ok := si.extractInstance(spec.ContainerRule, svc.ServiceInfo.Service, container); ok {
			instance.ContainerRule = spec.Name
			instName := instanceNameFor(container)
			err := si.Store.AddInstance(svc.Name, instName, instance)
			if err != nil {
				return err
			}
			svc.localInstances[instName] = struct{}{}
			log.Infof(`Registered %s instance '%.12s' at %s:%d`, svc.Name, instName, instance.Address, instance.Port)
			return nil
		}
	}
	return nil
}

// instanceNameFor and instanceNameFromEvent encode the fact we just
// use the container ID as the instance name.
func instanceNameFor(c *docker.Container) string {
	return c.ID
}

func (si *syncInstances) extractInstance(spec data.ContainerRule, svc data.Service, container *docker.Container) (data.Instance, bool) {
	var inst data.Instance
	if !spec.Includes(containerLabels{container}) {
		return inst, false
	}

	ipAddress, port := si.getAddress(spec, svc, container)
	if port == 0 {
		log.Infof(`Cannot extract address for instance, from container '%s'`, container.ID)
		inst.State = data.NOADDR
	} else {
		inst.Address = ipAddress
		inst.Port = port
		inst.State = data.LIVE
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
	inst.Labels = labels
	inst.Host = si.HostIP
	return inst, true
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
func (si *syncInstances) getAddress(spec data.ContainerRule, svc data.Service, container *docker.Container) (string, int) {
	if svc.InstancePort == 0 {
		return "", 0
	}
	if container.HostConfig.NetworkMode == "host" {
		return si.HostIP, svc.InstancePort
	}
	switch si.Network {
	case LOCAL:
		return si.mappedPortAddress(container, svc.InstancePort)
	case GLOBAL:
		return si.fixedPortAddress(container, svc.InstancePort)
	}
	return "", 0
}

/*
Extract a "mapped port" address. This mode assumes the balancer is
connecting to containers via a port "mapped" (NATed) by
Docker. Therefore it looks for the port mentioned in the list of
published ports, and finds the host port it has been mapped to. The IP
address is that given as the host's IP address.
*/
func (si *syncInstances) mappedPortAddress(container *docker.Container, port int) (string, int) {
	p := docker.Port(fmt.Sprintf("%d/tcp", port))
	if bindings, found := container.NetworkSettings.Ports[p]; found {
		for _, binding := range bindings {
			if binding.HostIP == si.HostIP || binding.HostIP == "" || binding.HostIP == "0.0.0.0" {
				mappedToPort, err := strconv.Atoi(binding.HostPort)
				if err != nil {
					return "", 0
				}
				return si.HostIP, mappedToPort
			}
		}
	}
	return "", 0
}

/*
Extract a "fixed port" address. This mode assumes that the balancer
will be able to connect to the container, potentially across hosts,
using the address Docker has assigned it.
*/
func (si *syncInstances) fixedPortAddress(container *docker.Container, port int) (string, int) {
	return container.NetworkSettings.IPAddress, port
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

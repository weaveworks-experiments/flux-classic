package agent

import (
	"fmt"
	"strconv"
	"strings"

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

type SyncInstances struct {
	store store.Store

	network    string
	services   map[string]*service
	containers map[string]*docker.Container
	hostIP     string
}

type Config struct {
	HostIP  string
	Network string
	Store   store.Store
}

func NewSyncInstances(config Config) *SyncInstances {
	listener := &SyncInstances{
		store:   config.Store,
		network: config.Network,
		hostIP:  config.HostIP,
	}
	return listener
}

func (si *SyncInstances) Run(containerUpdates <-chan ContainerUpdate, serviceUpdates <-chan store.ServiceUpdate) {
	for {
		select {
		case update := <-containerUpdates:
			si.processContainerUpdate(update)

		case update := <-serviceUpdates:
			si.processServiceUpdate(update)
		}
	}
}

func (si *SyncInstances) processContainerUpdate(update ContainerUpdate) {
	if update.Reset {
		si.containers = update.Containers
		for _, svc := range si.services {
			if err := si.syncInstances(svc); err != nil {
				log.Errorf(`Syncing instances for service '%s'`, svc.Name)
			}
		}

		return
	}

	for id, cont := range update.Containers {
		var err error
		var op string

		if cont != nil {
			si.containers[id] = cont
			err = si.addContainer(cont)
			op = "add"
		} else if cont := si.containers[id]; cont != nil {
			delete(si.containers, id)
			err = si.removeContainer(cont)
			op = "remove"
		}

		if err != nil {
			log.Errorf("Failed to %s container: %s", op, err)
		}
	}
}

func (si *SyncInstances) addContainer(container *docker.Container) error {
	for _, service := range si.services {
		log.Infof(`Evaluating container '%s' against service '%s'`, container.ID, service.Name)
		if err := si.evaluate(container, service); err != nil {
			return err
		}
	}
	return nil
}

func (si *SyncInstances) removeContainer(container *docker.Container) error {
	instName := instanceNameFor(container)
	for serviceName, svc := range si.services {
		if svc.includes(instName) {
			err := si.store.RemoveInstance(serviceName, instName)
			if err != nil {
				return err
			}
			log.Infof("Deregistered service '%s' instance '%.12s'", serviceName, instName)
			delete(svc.localInstances, instName)
		}
	}
	return nil
}

func (si *SyncInstances) processServiceUpdate(update store.ServiceUpdate) {
	if update.Reset {
		si.services = make(map[string]*service)

		for _, svcInfo := range update.Services {
			svc := si.redefineService(svcInfo)
			if err := si.syncInstances(svc); err != nil {
				log.Errorf(`Syncing instances for service '%s'`, svc.Name)
			}
		}

		return
	}

	for name, svcInfo := range update.Services {
		if svcInfo != nil {
			svc := si.redefineService(svcInfo)
			if err := si.syncInstances(svc); err != nil {
				log.Errorf(`Syncing instances for service '%s'`, svc.Name)
			}
		} else if svc := si.containers[name]; svc != nil {
			delete(si.services, name)
		}
	}
}

// The service has been changed; re-evaluate which containers belong,
// and which don't. Assume we have a correct list of containers.
func (si *SyncInstances) redefineService(svcInfo *store.ServiceInfo) *service {
	svc, found := si.services[svcInfo.Name]
	if !found {
		svc = &service{}
		si.services[svcInfo.Name] = svc
	}
	svc.ServiceInfo = svcInfo
	return svc
}

func (si *SyncInstances) syncInstances(svc *service) error {
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
	storeSvc, err := si.store.GetService(svc.Name, store.QueryServiceOptions{WithInstances: true})
	if err != nil {
		return err
	}

	for _, inst := range storeSvc.Instances {
		if !svc.includes(inst.Name) && si.owns(inst.Instance) {
			if err := si.store.RemoveInstance(svc.Name, inst.Name); err != nil {
				return err
			}
		}
	}

	return nil
}

func (si *SyncInstances) owns(inst data.Instance) bool {
	return si.hostIP == inst.Host
}

func (si *SyncInstances) evaluate(container *docker.Container, svc *service) error {
	for _, spec := range svc.ContainerRules {
		if instance, ok := si.extractInstance(spec.ContainerRule, svc.ServiceInfo.Service, container); ok {
			instance.ContainerRule = spec.Name
			instName := instanceNameFor(container)
			err := si.store.AddInstance(svc.Name, instName, instance)
			if err != nil {
				log.Errorf("Failed to register service: %s", err)
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

func (si *SyncInstances) extractInstance(spec data.ContainerRule, svc data.Service, container *docker.Container) (data.Instance, bool) {
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
	inst.Host = si.hostIP
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
func (si *SyncInstances) getAddress(spec data.ContainerRule, svc data.Service, container *docker.Container) (string, int) {
	if svc.InstancePort == 0 {
		return "", 0
	}
	if container.HostConfig.NetworkMode == "host" {
		return si.hostIP, svc.InstancePort
	}
	switch si.network {
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
func (si *SyncInstances) mappedPortAddress(container *docker.Container, port int) (string, int) {
	p := docker.Port(fmt.Sprintf("%d/tcp", port))
	if bindings, found := container.NetworkSettings.Ports[p]; found {
		for _, binding := range bindings {
			if binding.HostIP == si.hostIP || binding.HostIP == "" || binding.HostIP == "0.0.0.0" {
				mappedToPort, err := strconv.Atoi(binding.HostPort)
				if err != nil {
					return "", 0
				}
				return si.hostIP, mappedToPort
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
func (si *SyncInstances) fixedPortAddress(container *docker.Container, port int) (string, int) {
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

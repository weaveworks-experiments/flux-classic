package agent

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/squaremo/flux/common/daemon"
	"github.com/squaremo/flux/common/data"
	"github.com/squaremo/flux/common/store"

	log "github.com/Sirupsen/logrus"
	docker "github.com/fsouza/go-dockerclient"
)

type inspector interface {
	InspectContainer(string) (*docker.Container, error)
	ListContainers(docker.ListContainersOptions) ([]docker.APIContainers, error)
}

type instanceSet map[string]struct{}

type service struct {
	*store.ServiceInfo
	localInstances instanceSet
}

func (svc *service) includes(instanceName string) bool {
	_, ok := svc.localInstances[instanceName]
	return ok
}

func (svc *service) includeInstance(instanceName string) {
	svc.localInstances[instanceName] = struct{}{}
}

func (svc *service) excludeInstance(instanceName string) {
	delete(svc.localInstances, instanceName)
}

type Listener struct {
	store     store.Store
	inspector inspector

	services   map[string]*service
	containers map[string]*docker.Container
	hostIP     string
}

type Config struct {
	HostIP    string
	Store     store.Store
	Inspector inspector
}

func NewListener(config Config) *Listener {
	listener := &Listener{
		store:      config.Store,
		inspector:  config.Inspector,
		services:   make(map[string]*service),
		containers: make(map[string]*docker.Container),
		hostIP:     config.HostIP,
	}
	return listener
}

// A host identifier so we can tell which instances belong to this
// host when removing stale entries.
func (l *Listener) ownerID() string {
	return l.hostIP
}

func (l *Listener) owns(inst data.Instance) bool {
	return l.ownerID() == inst.OwnerID
}

// instanceNameFor and instanceNameFromEvent encode the fact we just
// use the container ID as the instance name.
func instanceNameFor(c *docker.Container) string {
	return c.ID
}

func instanceNameFromEvent(event *docker.APIEvents) string {
	return event.ID
}

// Read in all info on registered services and evaluate known
// containers against them.
func (l *Listener) ReadInServices() error {
	svcs, err := l.store.GetAllServices(store.QueryServiceOptions{WithContainerRules: true})
	if err != nil {
		return err
	}
	for _, svc := range svcs {
		if err := l.redefineService(svc.Name, svc); err != nil {
			return err
		}
	}
	return nil
}

// Read details of all running containers and evaluate against known
// services.
func (l *Listener) ReadExistingContainers() error {
	conts, err := l.inspector.ListContainers(docker.ListContainersOptions{})
	if err != nil {
		return err
	}
	for _, cont := range conts {
		if err := l.containerStarted(cont.ID); err != nil {
			return err
		}
	}
	return nil
}

// The service has been changed; re-evaluate which containers belong,
// and which don't. Assume we have a correct list of containers.
func (l *Listener) redefineService(serviceName string, newService store.ServiceInfo) error {
	svc, found := l.services[serviceName]
	if !found {
		svc = &service{}
		l.services[serviceName] = svc
	}
	svc.ServiceInfo = &newService
	svc.localInstances = make(instanceSet)
	var err error
	for _, container := range l.containers {
		if _, err = l.evaluate(container, svc); err != nil {
			return err
		}
	}
	// remove any instances for this service that do not match
	return store.ForeachInstance(l.store, serviceName, func(_, instanceName string, inst data.Instance) error {
		if !svc.includes(instanceName) && l.owns(inst) {
			return l.store.RemoveInstance(serviceName, instanceName)
		}
		return nil
	})
}

func (l *Listener) evaluate(container *docker.Container, svc *service) (bool, error) {
	for _, spec := range svc.ContainerRules {
		if instance, ok := l.extractInstance(spec.ContainerRule, container); ok {
			instance.ContainerRule = spec.Name
			instName := instanceNameFor(container)
			err := l.store.AddInstance(svc.Name, instName, instance)
			if err != nil {
				log.Errorf("Failed to register service: %s", err)
				return false, err
			}
			svc.includeInstance(instName)
			log.Infof(`Registered %s instance '%.12s' at %s:%d`, svc.Name, instName, instance.Address, instance.Port)
			return true, nil
		}
	}
	return false, nil
}

func (l *Listener) addContainer(container *docker.Container) error {
	l.containers[container.ID] = container
	for _, service := range l.services {
		log.Infof(`Evaluating container '%s' against service '%s'`, container.ID, service.Name)
		if _, err := l.evaluate(container, service); err != nil {
			return err
		}
	}
	return nil
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

func (l *Listener) extractInstance(spec data.ContainerRule, container *docker.Container) (data.Instance, bool) {
	var inst data.Instance
	if !spec.Includes(containerLabels{container}) {
		return inst, false
	}

	ipAddress, port := l.getAddress(spec, container)
	if port == 0 {
		log.Infof(`Cannot extract address for instance, from container '%s'`, container.ID)
		inst.State = data.NOADDR
	} else {
		inst.Address = ipAddress
		inst.Port = port
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
	inst.OwnerID = l.ownerID()
	return inst, true
}

func (l *Listener) removeContainer(instName string) error {
	for serviceName, svc := range l.services {
		if svc.includes(instName) {
			err := l.store.RemoveInstance(serviceName, instName)
			if err != nil {
				log.Errorf("Failed to deregister service: %s", err)
				return err
			}
			log.Infof("Deregistered service '%s' instance '%.12s'", serviceName, instName)
			svc.excludeInstance(instName)
		}
	}
	return nil
}

func (l *Listener) getAddress(spec data.ContainerRule, container *docker.Container) (string, int) {
	addrSpec := spec.AddressSpec
	switch addrSpec.Type {
	case data.MAPPED:
		return l.mappedPortAddress(container, addrSpec.Port)
	case data.FIXED:
		return l.fixedPortAddress(container, addrSpec.Port)
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
func (l *Listener) mappedPortAddress(container *docker.Container, port int) (string, int) {
	p := docker.Port(fmt.Sprintf("%d/tcp", port))
	if bindings, found := container.NetworkSettings.Ports[p]; found {
		for _, binding := range bindings {
			if binding.HostIP == l.hostIP || binding.HostIP == "" || binding.HostIP == "0.0.0.0" {
				mappedToPort, err := strconv.Atoi(binding.HostPort)
				if err != nil {
					return "", 0
				}
				return l.hostIP, mappedToPort
			}
		}
	}
	return "", 0
}

/*
Extract a "fixed port" address. This mode assumes that the balancer
will be able to connect to the container, potentially across hosts,
using the address Docker has assigned it.

There's a special case, which is if the container has been run with
`--net=host`; this means the container is using the host's networking
stack, so we should just use the host IP address.
*/
func (l *Listener) fixedPortAddress(container *docker.Container, port int) (string, int) {
	if container.HostConfig.NetworkMode == "host" {
		return l.hostIP, port
	}
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

// Deal with containers coming and going, and with configuration
// (service changes)

func (l *Listener) containerStarted(ID string) error {
	container, err := l.inspector.InspectContainer(ID)
	if err != nil {
		log.Errorf(`Failed to inspect container '%s': %s`, ID, err)
		return err
	}
	return l.addContainer(container)
}

func (l *Listener) serviceRemoved(name string) error {
	delete(l.services, name)
	return nil
}

func (l *Listener) serviceUpdated(name string) error {
	svc, err := l.store.GetService(name, store.QueryServiceOptions{WithContainerRules: true})
	if err != nil {
		log.Errorf(`Failed to retrieve service '%s': %s`, name, err)
		return err
	}

	log.Infof(`Service '%s' updated to %+v`, name, svc)
	// See which containers match now.
	return l.redefineService(name, svc)
}

func (l *Listener) Run(events <-chan *docker.APIEvents) {
	changes := make(chan data.ServiceChange)
	l.store.WatchServices(changes, nil, daemon.NewErrorSink(),
		store.QueryServiceOptions{WithContainerRules: true})
	for {
		select {
		case event := <-events:
			l.processDockerEvent(event)
		case change := <-changes:
			l.processServiceChange(change)
		}
	}
}

func (l *Listener) processDockerEvent(event *docker.APIEvents) {
	switch event.Status {
	case "start":
		if err := l.containerStarted(event.ID); err != nil {
			log.Errorf("error handling container start: %s", err)
		}
	case "die":
		if err := l.removeContainer(instanceNameFromEvent(event)); err != nil {
			log.Errorf("error handling container die: %s", err)
		}
	}
}

func (l *Listener) processServiceChange(change data.ServiceChange) {
	if change.ServiceDeleted {
		if err := l.serviceRemoved(change.Name); err != nil {
			log.Errorf("error handling service removal: %s", err)
		}
	} else {
		if err := l.serviceUpdated(change.Name); err != nil {
			log.Errorf("error handling service update: %s", err)
		}
	}
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

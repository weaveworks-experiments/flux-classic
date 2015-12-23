package agent

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/squaremo/ambergreen/common/daemon"
	"github.com/squaremo/ambergreen/common/data"
	"github.com/squaremo/ambergreen/common/store"

	docker "github.com/fsouza/go-dockerclient"
)

type inspector interface {
	InspectContainer(string) (*docker.Container, error)
	ListContainers(docker.ListContainersOptions) ([]docker.APIContainers, error)
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

type service struct {
	name       string
	details    data.Service
	groupSpecs map[string]data.ContainerGroupSpec
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

func instanceNameFor(c *docker.Container) string {
	return c.ID
}

// Read in all info on registered services
func (l *Listener) ReadInServices() error {
	err := l.store.ForeachServiceInstance(func(name string, value data.Service) {
		l.services[name] = &service{name: name, details: value}
	}, nil)
	if err != nil {
		return err
	}

	for name := range l.services {
		specs, err := l.store.GetContainerGroupSpecs(name)
		if err != nil {
			return err
		}

		l.services[name].groupSpecs = specs
	}

	return nil
}

// Read details of all running containers
func (l *Listener) ReadExistingContainers() error {
	conts, err := l.inspector.ListContainers(docker.ListContainersOptions{})
	if err != nil {
		return err
	}
	for _, cont := range conts {
		container, err := l.inspector.InspectContainer(cont.ID)
		if err != nil {
			log.Println("Failed to inspect container:", cont.ID, err)
			continue
		}
		l.containers[instanceNameFor(container)] = container
	}
	return nil
}

// Assume we know all of the services, and all of the containers, and
// make sure the matching instances (and only the matching instances)
// are recorded.
func (l *Listener) reconcile() error {
	// Register all the  we know about
	for _, container := range l.containers {
		l.matchContainer(container)
	}
	// Remove instances for which there is no longer a running
	// container
	return l.store.ForeachServiceInstance(nil, func(serviceName string, instanceName string, inst data.Instance) {
		if _, found := l.containers[instanceName]; !found && l.owns(inst) {
			log.Printf("Removing %.12s/%.12s", serviceName, instanceName)
			l.store.RemoveInstance(serviceName, instanceName)
		}
	})
}

// The service has been changed; re-evaluate which containers belong,
// and which don't. Assume we have a correct list of containers.
func (l *Listener) redefineService(serviceName string, service *service) error {
	keep := make(map[string]struct{})
	var (
		inService bool
		err       error
	)
	for _, container := range l.containers {
		if inService, err = l.evaluate(container, service); err != nil {
			return err
		}
		if inService {
			keep[instanceNameFor(container)] = struct{}{}
		}
	}
	// remove any instances for this service that do not match
	return l.store.ForeachInstance(serviceName, func(instanceName string, _ data.Instance) {
		if _, found := keep[instanceName]; !found {
			l.store.RemoveInstance(serviceName, instanceName)
		}
	})
}

func (l *Listener) evaluate(container *docker.Container, service *service) (bool, error) {
	for group, spec := range service.groupSpecs {
		if instance, ok := l.extractInstance(spec, container); ok {
			instance.ContainerGroup = group
			err := l.store.AddInstance(service.name, container.ID, instance)
			if err != nil {
				log.Println("Failed to register service:", err)
				return false, err
			}
			log.Printf("Registered %s instance %.12s at %s:%d", service.name, container.ID, instance.Address, instance.Port)
			return true, nil
		}
	}
	return false, nil
}

func (l *Listener) matchContainer(container *docker.Container) error {
	for _, service := range l.services {
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

func (l *Listener) extractInstance(spec data.ContainerGroupSpec, container *docker.Container) (data.Instance, bool) {
	if !spec.Includes(containerLabels{container}) {
		return data.Instance{}, false
	}

	ipAddress, port := l.getAddress(spec, container)
	if port == 0 {
		log.Printf("Cannot extract instance from container '%s', no address extractable from %+v\n", container.ID, container.NetworkSettings)
		return data.Instance{}, false
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

	return data.Instance{
		OwnerID: l.ownerID(),
		Address: ipAddress,
		Port:    port,
		Labels:  labels,
	}, true
}

func (l *Listener) deregister(container *docker.Container) error {
	for serviceName, _ := range l.services {
		if l.store.CheckRegisteredService(serviceName) == nil {
			err := l.store.RemoveInstance(serviceName, container.ID)
			if err != nil {
				log.Println("Failed to deregister service:", err)
				return err
			}
			log.Printf("Deregistered %s instance %.12s", serviceName, container.ID)
		}
	}
	return nil
}

func (l *Listener) getAddress(spec data.ContainerGroupSpec, container *docker.Container) (string, int) {
	addrSpec := spec.AddressSpec
	switch addrSpec.Type {
	case data.MAPPED:
		return l.mappedPortAddress(container, addrSpec.Port)
	case data.FIXED:
		return l.fixedPortAddress(container, addrSpec.Port)
	}
	return "", 0
}

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

func (l *Listener) fixedPortAddress(container *docker.Container, port int) (string, int) {
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
		log.Println("Failed to inspect container:", ID, err)
		return err
	}
	l.containers[ID] = container
	return l.matchContainer(container)
}

func (l *Listener) containerDied(ID string) error {
	container, found := l.containers[ID]
	if !found {
		log.Println("Unknown container:", ID)
		return nil
	}
	return l.deregister(container)
}

func (l *Listener) serviceRemoved(name string) error {
	delete(l.services, name)
	return nil
}

func (l *Listener) serviceUpdated(name string) error {
	svc, err := l.store.GetServiceDetails(name)
	if err != nil {
		log.Println("Failed to retrieve service:", name, err)
		return err
	}

	specs, err := l.store.GetContainerGroupSpecs(name)
	if err != nil {
		return err
	}

	s := &service{name, svc, specs}
	l.services[name] = s
	log.Println("Service", name, "updated:", svc)
	// See which containers match now.
	return l.redefineService(name, s)
}

func (l *Listener) Run(events <-chan *docker.APIEvents) {
	changes := make(chan data.ServiceChange)
	l.store.WatchServices(changes, nil, daemon.NewErrorSink(),
		store.WatchServicesOptions{WithGroupSpecChanges: true})

	// sync after we have initiated the watch
	if err := l.reconcile(); err != nil {
		log.Fatal("Error synchronising existing containers:", err)
	}

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
			log.Println("error handling container start: ", err)
		}
	case "die":
		if err := l.containerDied(event.ID); err != nil {
			log.Println("error handling container die: ", err)
		}
	}
}

func (l *Listener) processServiceChange(change data.ServiceChange) {
	if change.ServiceDeleted {
		if err := l.serviceRemoved(change.Name); err != nil {
			log.Println("error handling service removal: ", err)
		}
	} else {
		if err := l.serviceUpdated(change.Name); err != nil {
			log.Println("error handling service update: ", err)
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

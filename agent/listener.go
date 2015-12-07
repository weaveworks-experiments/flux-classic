package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/squaremo/ambergreen/common/backends"
	"github.com/squaremo/ambergreen/common/data"

	docker "github.com/fsouza/go-dockerclient"
)

type Listener struct {
	backend    *backends.Backend
	dc         *docker.Client
	services   map[string]*service
	containers map[string]*docker.Container
	hostIP     string
}

type Config struct {
	HostIP string
}

type service struct {
	name    string
	details data.Service
}

func NewListener(config Config, dc *docker.Client) *Listener {
	listener := &Listener{
		backend:    backends.NewBackendFromEnv(),
		dc:         dc,
		services:   make(map[string]*service),
		containers: make(map[string]*docker.Container),
		hostIP:     config.HostIP,
	}
	return listener
}

// Read in all info on registered services
func (l *Listener) ReadInServices() error {
	return l.backend.ForeachServiceInstance(func(name string, value data.Service) {
		l.services[name] = &service{name: name, details: value}
	}, nil)
}

// Read details of all running containers
func (l *Listener) ReadExistingContainers() error {
	conts, err := l.dc.ListContainers(docker.ListContainersOptions{})
	if err != nil {
		return err
	}
	for _, cont := range conts {
		container, err := l.dc.InspectContainer(cont.ID)
		if err != nil {
			log.Println("Failed to inspect container:", cont.ID, err)
			continue
		}
		l.containers[cont.ID] = container
	}
	return nil
}

// TODO: Un-enrol ones that no longer match.  If required.
func (l *Listener) Sync() error {
	// Register all the ones we know about
	for _, container := range l.containers {
		l.Register(container)
	}
	// Remove all the ones we don't
	return l.backend.ForeachServiceInstance(nil, func(serviceName string, instanceName string, _ data.Instance) {
		if _, found := l.containers[instanceName]; !found {
			log.Printf("Removing %.12s/%.12s", serviceName, instanceName)
			l.backend.RemoveInstance(serviceName, instanceName)
		}
	})
}

func (l *Listener) Register(container *docker.Container) error {
nextService:
	for serviceName, service := range l.services {
		for group, spec := range service.details.InstanceSpecs {
			if instance, ok := l.extractInstance(spec, container); ok {
				instance.InstanceGroup = group
				err := l.backend.AddInstance(serviceName, container.ID, instance)
				if err != nil {
					log.Println("ambergreen: failed to register service:", err)
					return err
				}
				log.Printf("Registered %s instance %.12s at %s:%d", serviceName, container.ID, instance.Address, instance.Port)
				continue nextService
			}
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

func (l *Listener) extractInstance(spec data.InstanceSpec, container *docker.Container) (data.Instance, bool) {
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
		Address: ipAddress,
		Port:    port,
		Labels:  labels,
	}, true
}

func (l *Listener) Deregister(container *docker.Container) error {
	for serviceName, _ := range l.services {
		if l.backend.CheckRegisteredService(serviceName) == nil {
			err := l.backend.RemoveInstance(serviceName, container.ID)
			if err != nil {
				log.Println("ambergreen: failed to deregister service:", err)
				return err
			}
			log.Printf("Deregistered %s instance %.12s", serviceName, container.ID)
		}
	}
	return nil
}

func (l *Listener) getAddress(spec data.InstanceSpec, container *docker.Container) (string, int) {
	addrSpec := spec.AddressSpec
	switch addrSpec.Type {
	case "mapped":
		return l.mappedPortAddress(container, addrSpec.Port)
	case "fixed":
		return l.fixedPortAddress(container, addrSpec.Port)
	}
	return "", 0
}

func (l *Listener) mappedPortAddress(container *docker.Container, port int) (string, int) {
	p := docker.Port(fmt.Sprintf("%d/tcp", port))
	if bindings, found := container.NetworkSettings.Ports[p]; found {
		for _, binding := range bindings {
			if binding.HostIP == l.hostIP || binding.HostIP == "" || binding.HostIP == "0.0.0.0" {
				port, err := strconv.Atoi(binding.HostPort)
				if err != nil {
					return "", 0
				}
				return l.hostIP, port
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

func (l *Listener) Run(events <-chan *docker.APIEvents) {
	changes := make(chan data.ServiceChange)
	l.backend.WatchServices(changes, nil, false)

	// sync after we have initiated the watch
	if err := l.Sync(); err != nil {
		log.Fatal("Error synchronising existing containers:", err)
	}

	for {
		select {
		case event := <-events:
			switch event.Status {
			case "start":
				container, err := l.dc.InspectContainer(event.ID)
				if err != nil {
					log.Println("Failed to inspect container:", event.ID, err)
					continue
				}
				l.containers[event.ID] = container
				l.Register(container)
			case "die":
				container, found := l.containers[event.ID]
				if !found {
					log.Println("Unknown container:", event.ID)
					continue
				}
				l.Deregister(container)
			}
		case change := <-changes:
			if change.Deleted {
				delete(l.services, change.Name)
				log.Println("Service deleted:", change.Name)
			} else {
				svc, err := l.backend.GetServiceDetails(change.Name)
				if err != nil {
					log.Println("Failed to retrieve service:", change.Name, err)
					continue
				}

				l.services[change.Name] = &service{change.Name, svc}
				log.Println("Service", change.Name, "updated:", svc)

				// See if any containers match now.
				l.Sync()
			}
		}
	}
}

func imageTag(image string) string {
	colon := strings.LastIndex(image, ":")
	if colon == -1 {
		return "latest"
	}
	return image[colon:]
}

func imageName(image string) string {
	colon := strings.LastIndex(image, ":")
	if colon == -1 {
		return image
	}
	return image[:colon]
}

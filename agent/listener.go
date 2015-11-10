package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/squaremo/ambergreen/pkg/backends"
	"github.com/squaremo/ambergreen/pkg/data"

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
		backend:    backends.NewBackend([]string{}),
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
	var serviceName string
	return l.backend.ForeachServiceInstance(func(name string, _ data.Service) {
		serviceName = name
	}, func(instanceName string, _ data.Instance) {
		if _, found := l.containers[instanceName]; !found {
			log.Printf("Removing %.12s/%.12s", serviceName, instanceName)
			l.backend.RemoveInstance(serviceName, instanceName)
		}
	})
}

func (l *Listener) Register(container *docker.Container) error {
	for serviceName, service := range l.services {
		spec := service.details.InstanceSpec
		if instance, ok := l.extractInstance(spec, container); ok {
			err := l.backend.AddInstance(serviceName, container.ID, instance)
			if err != nil {
				log.Println("ambergreen: failed to register service:", err)
				return err
			}
			log.Printf("Registered %s instance %.12s at %s:%d", serviceName, container.ID, instance.Address, instance.Port)
		}
	}
	return nil
}

func (l *Listener) extractInstance(spec data.InstanceSpec, container *docker.Container) (data.Instance, bool) {
	if !l.includesContainer(spec, container) {
		return data.Instance{}, false
	}

	ipAddress, port := l.getAddress(spec, container)
	labels := map[string]string{
		"docker.io/tag":   imageTag(container.Config.Image),
		"docker.io/image": imageName(container.Config.Image),
	}
	for k, v := range container.Config.Labels {
		labels[k] = v
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
				log.Println("coatl: failed to deregister service:", err)
				return err
			}
			log.Printf("Deregistered %s instance %.12s", serviceName, container.ID)
		}
	}
	return nil
}

func (l *Listener) includesContainer(spec data.InstanceSpec, container *docker.Container) bool {
	for label, value := range spec.Selector {
		switch {
		case label == "image":
			if imageName(container.Config.Image) != value {
				return false
			}
		case len(label) > 4 && label[:4] == "env.":
			if envValue(container.Config.Env, label[4:]) != value {
				return false
			}
		default:
			if container.Config.Labels[label] != value {
				return false
			}
		}
	}
	return true
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
			if binding.HostIP == "" || binding.HostIP == "0.0.0.0" {
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
	backendCh := l.backend.Watch()
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
		case r := <-backendCh:
			serviceName, instanceName, err := data.DecodePath(r.Node.Key)
			if err != nil {
				log.Println(err)
				continue
			}
			switch {
			case r.Action == "delete" && serviceName == "":
				// everything deleted
				l.services = make(map[string]*service)
				log.Println("All services deleted")
			case r.Action == "delete" && instanceName == "":
				delete(l.services, serviceName)
				log.Println("Service deleted:", serviceName)
			case r.Action == "set" && instanceName == "details":
				s := &service{name: serviceName, details: data.Service{}}
				if err := json.Unmarshal([]byte(r.Node.Value), &s.details); err != nil {
					log.Println("Error unmarshalling: ", err)
					continue
				}
				l.services[serviceName] = &service{name: serviceName, details: data.Service{}}
				log.Println("Service", s.name, "updated:", s.details)
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

package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/spf13/cobra"
	"github.com/squaremo/ambergreen/common/backends"
	"github.com/squaremo/ambergreen/common/data"
)

var topCmd = &cobra.Command{
	Use:   "listen",
	Short: "listen to weave Run updates",
	Long:  `Write more documentation here`,
	Run:   run,
}

var backend *backends.Backend

func main() {
	backend = backends.NewBackend([]string{})

	err := topCmd.Execute()
	if err != nil {
		log.Fatal(err)
	}
}

const (
	servicePath = "/weave/service/"
)

type instance struct {
	name    string
	details data.Instance
}

type service struct {
	name      string
	details   data.Service
	instances map[string]*instance
}

var services map[string]*service

func createService(name string, details data.Service) *service {
	s := &service{name: name, details: details, instances: make(map[string]*instance)}
	services[name] = s
	return s
}

func createInstance(s *service, name string, details data.Instance) *instance {
	i := &instance{name: name, details: details}
	s.instances[i.name] = i
	return i
}

func initialize() {
	services = make(map[string]*service)
	var s *service
	backend.ForeachServiceInstance(func(name string, serviceData data.Service) {
		s = createService(name, serviceData)
	}, func(name string, instanceData data.Instance) {
		createInstance(s, name, instanceData)
	})
}

func run(cmd *cobra.Command, args []string) {
	initialize()
	fmt.Print(len(services), " services:")
	for name := range services {
		fmt.Print(" ", name)
	}
	fmt.Println()
	ch := backend.Watch()

	for r := range ch {
		//fmt.Println(r.Action, r.Node)
		serviceName, instanceName, err := data.DecodePath(r.Node.Key)
		if err != nil {
			log.Println(err)
			continue
		}
		switch r.Action {
		case "create":
			createService(serviceName, data.Service{})
			fmt.Println("Service created:", serviceName, "; there are now", len(services), "services")
		case "delete":
			if serviceName == "" {
				// everything deleted
				services = make(map[string]*service)
				fmt.Println("All services deleted")
			} else if instanceName == "" {
				delete(services, serviceName)
				fmt.Println("Service deleted:", serviceName, "; there are now", len(services), "services")
			} else {
				s, ok := services[serviceName]
				if !ok {
					log.Println("Service not found:", serviceName)
					continue
				}
				delete(s.instances, instanceName)
				fmt.Println("Instance", instanceName, "removed from", s.name, "which now has", len(s.instances), "instances")
			}
		case "set":
			s, ok := services[serviceName]
			if !ok {
				log.Println("Service not found:", serviceName)
				continue
			}
			if instanceName == "details" {
				if err := json.Unmarshal([]byte(r.Node.Value), &s.details); err != nil {
					log.Println("Error unmarshalling: ", err)
					continue
				}
				fmt.Println("Service", s.name, s.details)
			} else {
				var details data.Instance
				if err := json.Unmarshal([]byte(r.Node.Value), &details); err != nil {
					log.Fatal("Error unmarshalling: ", err)
				}
				i := createInstance(s, instanceName, details)
				fmt.Println("Instance", i.name, "is now enrolled in", s.name, "which now has", len(s.instances), "instances")
			}
		default:
			fmt.Println("Unhandled:", r.Action, r.Node)
		}
	}
}

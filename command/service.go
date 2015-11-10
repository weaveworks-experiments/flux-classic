package main

import (
	"fmt"
	"log"
	"strconv"

	"github.com/squaremo/ambergreen/pkg/data"
)

// example: coatlctl service create --docker-image micro-wiki/pages
type addServiceOpts struct {
	dockerImage string
	protocol    string
}

const DEFAULT_GROUP = data.InstanceGroup("default")

func (opts *addServiceOpts) addService(args []string) {
	if len(args) != 3 {
		log.Fatal("Must supply service name, address and port number")
	}
	serviceName := args[0]
	port, err := strconv.Atoi(args[2])
	if err != nil {
		log.Fatal("Invalid port number:", err)
	}

	err = backend.AddService(serviceName, data.Service{
		Address:  args[1],
		Port:     port,
		Protocol: opts.protocol,
		InstanceSpecs: map[data.InstanceGroup]data.InstanceSpec{
			DEFAULT_GROUP: data.InstanceSpec{
				AddressSpec: data.AddressSpec{
					Type: "fixed",
					Port: port,
				},
				Selector: map[string]string{
					"image": opts.dockerImage,
				},
			},
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Added service:", serviceName)
}

type listServiceOpts struct {
	all bool
}

func (opts *listServiceOpts) listService(args []string) {
	printService := func(name string, value data.Service) { fmt.Println(name, value) }
	var printInstance func(name string, value data.Instance)
	if opts.all {
		printInstance = func(name string, value data.Instance) { fmt.Println("  ", name, value) }
	}
	err := backend.ForeachServiceInstance(printService, printInstance)
	if err != nil {
		log.Fatal(err)
	}
}

func (opts *listServiceOpts) removeService(args []string) {
	var err error
	if opts.all {
		err = backend.RemoveAllServices()
	} else if len(args) == 1 {
		err = backend.RemoveService(args[0])
	} else {
		log.Fatal("Must supply service name or -a")
	}
	if err != nil {
		log.Fatal("Failed to delete:", err)
	}
}

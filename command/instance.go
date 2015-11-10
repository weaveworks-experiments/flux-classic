package main

import (
	"fmt"
	"log"
	"strconv"

	"github.com/squaremo/ambergreen/pkg/data"
)

func enrol(args []string) {
	if len(args) != 4 {
		log.Fatal("Usage: amberctl enrol <service> <instance> <address> <port>")
	}
	serviceName, instance := args[0], args[1]
	if err := backend.CheckRegisteredService(serviceName); err != nil {
		log.Fatal("Cannot find service '", serviceName, "':", err)
	}
	port, err := strconv.Atoi(args[3])
	if err != nil {
		log.Fatal("Invalid port number: ", err)
	}
	err = backend.AddInstance(serviceName, instance, data.Instance{
		Address: args[2],
		Port:    port,
		Labels:  map[string]string{},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Enrolled", instance, "in service", serviceName)
}

func unenrol(args []string) {
	if len(args) != 2 {
		log.Fatal("Usage: amberctl unenrol <service> <instance>")
	}
	serviceName, instance := args[0], args[1]
	err := backend.RemoveInstance(serviceName, instance)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Un-enrolled", instance, "from service", serviceName)
}

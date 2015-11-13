package main

import (
	"fmt"
	"log"
	"net"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/squaremo/ambergreen/common/backends"
	"github.com/squaremo/ambergreen/common/data"
)

const DEFAULT_GROUP = data.InstanceGroup("default")

type addOpts struct {
	selectOpts
}

func addServiceCommands(top *cobra.Command, backend *backends.Backend) {
	add := addOpts{}
	add.backend = backend
	add.addCommandTo(top)
	list := listOpts{backend: backend}
	list.addCommandTo(top)
	rm := rmOpts{backend: backend}
	rm.addCommandTo(top)
}

func (opts *addOpts) addCommandTo(top *cobra.Command) {
	addCmd := &cobra.Command{
		Use:   "service <name> <IP address> <port> [options]",
		Short: "define a service",
		Long:  "Define a service, optionally giving a default specification for instances belonging to that service.",
		Run:   opts.run,
	}
	opts.AddVars(addCmd)
	top.AddCommand(addCmd)
}

func (opts *addOpts) run(cmd *cobra.Command, args []string) {
	if len(args) < 3 {
		exitWithErrorf("Expected arguments <name>, <IP address>, <port>")
	}
	serviceName := args[0]
	port, err := strconv.Atoi(args[2])
	if err != nil {
		exitWithErrorf("Invalid port number: " + err.Error())
	}
	ip := net.ParseIP(args[1])
	if ip == nil {
		exitWithErrorf("invalid IP address: ", args[1])
	}

	instSpecs := make(map[data.InstanceGroup]data.InstanceSpec)
	if spec, err := opts.makeSpec(); err == nil {
		if spec != nil {
			instSpecs[DEFAULT_GROUP] = *spec
		}
	} else {
		exitWithErrorf("Unable to extract spec from opitions: ", err)
	}

	err = opts.backend.AddService(serviceName, data.Service{
		Address:       args[1],
		Port:          port,
		Protocol:      opts.protocol,
		InstanceSpecs: instSpecs,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Added service:", serviceName)
}

type listOpts struct {
	verbose bool

	backend *backends.Backend
}

func (opts *listOpts) addCommandTo(top *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "list [options]",
		Short: "list the services defined",
		Run:   opts.run,
	}
	cmd.Flags().BoolVar(&opts.verbose, "verbose", false, "print the instances for each service in the list")
	top.AddCommand(cmd)
}

func (opts *listOpts) run(_ *cobra.Command, args []string) {
	printService := func(name string, value data.Service) { fmt.Println(name, value) }
	var printInstance func(name string, value data.Instance)
	if opts.verbose {
		printInstance = func(name string, value data.Instance) { fmt.Println("  ", name, value) }
	}
	err := opts.backend.ForeachServiceInstance(printService, printInstance)
	if err != nil {
		exitWithErrorf("Unable to enumerate services: ", err)
	}
}

type rmOpts struct {
	all bool

	backend *backends.Backend
}

func (opts *rmOpts) addCommandTo(top *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "rm <service>|--all",
		Short: "remove service definition(s)",
		Run:   opts.run,
	}
	cmd.Flags().BoolVar(&opts.all, "all", false, "remove all service definitions")
	top.AddCommand(cmd)
}

func (opts *rmOpts) run(_ *cobra.Command, args []string) {
	var err error
	if opts.all {
		err = opts.backend.RemoveAllServices()
	} else if len(args) == 1 {
		err = opts.backend.RemoveService(args[0])
	} else {
		exitWithErrorf("Must supply service name or --all")
	}
	if err != nil {
		exitWithErrorf("Failed to delete: " + err.Error())
	}
}

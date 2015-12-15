package main

import (
	"fmt"
	"net"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/squaremo/ambergreen/common/data"
	"github.com/squaremo/ambergreen/common/store"
)

const DEFAULT_GROUP = string("default")

type addOpts struct {
	store store.Store

	spec
}

func (opts *addOpts) addCommandTo(top *cobra.Command) {
	addCmd := &cobra.Command{
		Use:   "service <name> <IP address> <port> [options]",
		Short: "define a service",
		Long:  "Define a service, optionally giving a default specification for instances belonging to that service.",
		Run:   opts.run,
	}
	opts.addSpecVars(addCmd)
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

	err = opts.store.AddService(serviceName, data.Service{
		Address:  args[1],
		Port:     port,
		Protocol: opts.protocol,
	})
	if err != nil {
		exitWithErrorf("Error updating service: ", err)
	}

	spec, err := opts.makeSpec()
	if err != nil {
		exitWithErrorf("Unable to extract spec from opitions: ", err)
	}

	if err = opts.store.SetContainerGroupSpec(serviceName, DEFAULT_GROUP, *spec); err != nil {
		exitWithErrorf("Error updating service: ", err)
	}

	fmt.Println("Added service:", serviceName)
}

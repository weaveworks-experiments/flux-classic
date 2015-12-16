package main

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/squaremo/ambergreen/common/data"
	"github.com/squaremo/ambergreen/common/store"
)

const DEFAULT_GROUP = string("default")

type addOpts struct {
	store store.Store

	spec
	protocol string
}

func (opts *addOpts) addCommandTo(top *cobra.Command) {
	addCmd := &cobra.Command{
		Use:   "service name ipaddress:port",
		Short: "define a service",
		Long:  "Define a service, optionally giving a default specification for containers belonging to that service.",
		Run:   opts.run,
	}
	addCmd.Flags().StringVarP(&opts.protocol, "protocol", "p", "tcp", `the protocol to assume for connections to the service; either "http" or "tcp"`)
	opts.addSpecVars(addCmd)
	top.AddCommand(addCmd)
}

func (opts *addOpts) run(cmd *cobra.Command, args []string) {
	if len(args) < 2 {
		exitWithErrorf("Expected arguments <name> <ipaddress:port>")
	}
	serviceName := args[0]
	addr := strings.Split(args[1], ":")
	if len(addr) != 2 {
		exitWithErrorf("Expected second argument in form address:port")
	}
	port, err := strconv.Atoi(addr[1])
	// We may later use 0 to mean "please allocate"
	if err != nil || port < 1 || port > 65535 {
		exitWithErrorf("Invalid port number (: " + err.Error())
	}
	ip := net.ParseIP(addr[0])
	if ip == nil {
		exitWithErrorf("Invalid IP address: ", args[1])
	}

	err = opts.store.AddService(serviceName, data.Service{
		Address:  addr[0],
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

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
	address  string
	protocol string
}

func (opts *addOpts) addCommandTo(top *cobra.Command) {
	addCmd := &cobra.Command{
		Use:   "service <name>",
		Short: "define a service",
		Long:  "Define service <name>, optionally giving an address at which it can be reached on each host, and optionally giving a specification for enrolling containers in the service.",
		Run:   opts.run,
	}
	addCmd.Flags().StringVar(&opts.address, "address", "", "in the format <ipaddr>:<port>[/<protocol>], the IP address and port at which the service should be made available on each host; optionally, the protocol to assume.")
	addCmd.Flags().StringVarP(&opts.protocol, "protocol", "p", "", `the protocol to assume for connections to the service; either "http" or "tcp". Overrides the protocol given in --address if present.`)
	opts.addSpecVars(addCmd)
	top.AddCommand(addCmd)
}

func (opts *addOpts) run(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		exitWithErrorf("Expected argument <name>")
	}
	serviceName := args[0]

	var (
		port     int
		ipaddr   string
		protocol string = "tcp"
	)

	if opts.address != "" {
		addr := strings.Split(opts.address, ":")
		if len(addr) != 2 {
			exitWithErrorf("Expected address in the format <ipaddress>:<port>[/<protocol>]")
		}

		ip := net.ParseIP(addr[0])
		if ip == nil {
			exitWithErrorf("Invalid IP address: ", args[1])
		}
		ipaddr = addr[0]

		portProt := strings.SplitN(addr[1], "/", 2)
		port, err := strconv.Atoi(portProt[0])
		// We may later use 0 to mean "please allocate"
		if err != nil {
			exitWithErrorf("Invalid port number:", err.Error())
		} else if port < 1 || port > 65535 {
			exitWithErrorf("Invalid port number; expected 0 < p < 65535, got %d", port)
		}

		if len(portProt) == 2 {
			protocol = portProt[1]
		}
		if opts.protocol != "" {
			protocol = opts.protocol
		}
	}

	err := opts.store.AddService(serviceName, data.Service{
		Address:  ipaddr,
		Port:     port,
		Protocol: protocol,
	})
	if err != nil {
		exitWithErrorf("Error updating service: ", err)
	}

	spec, err := opts.makeSpec()
	if err != nil {
		exitWithErrorf("Unable to extract spec from options: ", err)
	}

	if spec != nil {
		if err = opts.store.SetContainerGroupSpec(serviceName, DEFAULT_GROUP, *spec); err != nil {
			exitWithErrorf("Error updating service: ", err)
		}
	}

	fmt.Println("Added service:", serviceName)
}

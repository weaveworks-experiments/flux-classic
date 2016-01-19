package main

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/squaremo/flux/common/data"
)

const DEFAULT_GROUP = string("default")

type addOpts struct {
	baseOpts
	spec

	address  string
	protocol string
}

func (opts *addOpts) makeCommand() *cobra.Command {
	addCmd := &cobra.Command{
		Use:   "service <name>",
		Short: "define a service",
		Long:  "Define service <name>, optionally giving an address at which it can be reached on each host, and optionally giving a rule for selecting containers as instances of the service.",
		RunE:  opts.run,
	}
	addCmd.Flags().StringVar(&opts.address, "address", "", "in the format <ipaddr>:<port>[/<protocol>], the IP address and port at which the service should be made available on each host; optionally, the protocol to assume.")
	addCmd.Flags().StringVarP(&opts.protocol, "protocol", "p", "", `the protocol to assume for connections to the service; either "http" or "tcp". Overrides the protocol given in --address if present.`)
	opts.addSpecVars(addCmd)
	return addCmd
}

func parseAddress(address string) (svc data.Service, err error) {
	if address == "" {
		return
	}

	addr := strings.Split(address, ":")
	if len(addr) != 2 {
		err = fmt.Errorf("Expected address in the format <ipaddress>:<port>[/<protocol>]")
		return
	}

	ip := net.ParseIP(addr[0])
	if ip == nil {
		err = fmt.Errorf("Invalid IP address: ", addr[0])
		return
	}
	svc.Address = addr[0]

	var port int
	portProt := strings.SplitN(addr[1], "/", 2)
	port, err = strconv.Atoi(portProt[0])
	// We may later use 0 to mean "please allocate"
	if err != nil {
		return
	} else if port < 1 || port > 65535 {
		err = fmt.Errorf("Invalid port number; expected 0 < p < 65535, got %d", port)
		return
	}
	svc.Port = port

	if len(portProt) == 2 {
		svc.Protocol = portProt[1]
	}
	return
}

func (opts *addOpts) run(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("Expected argument <name>")
	}
	serviceName := args[0]

	svc, err := parseAddress(opts.address)
	if opts.protocol != "" {
		svc.Protocol = opts.protocol
	}

	err = opts.store.AddService(serviceName, svc)
	if err != nil {
		return fmt.Errorf("Error updating service: ", err)
	}

	spec, err := opts.makeSpec()
	if err != nil {
		return fmt.Errorf("Unable to extract rule from options: ", err)
	}

	if spec != nil {
		if err = opts.store.SetContainerRule(serviceName, DEFAULT_GROUP, *spec); err != nil {
			return fmt.Errorf("Error updating service: ", err)
		}
	}

	fmt.Fprintln(opts.getStdout(), serviceName)
	return nil
}

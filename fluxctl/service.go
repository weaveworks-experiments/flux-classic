package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/weaveworks/flux/common/data"
	"github.com/weaveworks/flux/common/netutil"
)

const DEFAULT_GROUP = string("default")

type addOpts struct {
	baseOpts
	spec

	address      string
	instancePort int
	protocol     string
}

func (opts *addOpts) makeCommand() *cobra.Command {
	addCmd := &cobra.Command{
		Use:   "service <name>",
		Short: "define a service",
		Long:  "Define service <name>, optionally giving an address at which it can be reached on each host, and optionally giving a rule for selecting containers as instances of the service.",
		RunE:  opts.run,
	}
	addCmd.Flags().StringVar(&opts.address, "address", "", "in the format <ipaddr>:<port>, the IP address and port at which the service should be made available on each host.")
	addCmd.Flags().StringVarP(&opts.protocol, "protocol", "p", "", `the protocol to assume for connections to the service; either "http" or "tcp". Overrides the protocol given in --address if present.`)
	addCmd.Flags().IntVar(&opts.instancePort, "instance-port", 0, "port to use for instance addresses (if not the same as in the service address).")
	opts.addSpecVars(addCmd)
	return addCmd
}

func parseAddress(address string) (data.Service, error) {
	var svc data.Service
	if address == "" {
		return svc, nil
	}

	var err error
	svc.Address, svc.Port, err = netutil.SplitAddressPort(address, "", false)
	return svc, err
}

func (opts *addOpts) run(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("Expected argument <name>")
	}
	serviceName := args[0]

	svc, err := parseAddress(opts.address)

	if opts.protocol != "" {
		svc.Protocol = opts.protocol
	}
	if opts.instancePort == 0 {
		svc.InstancePort = svc.Port
	} else {
		svc.InstancePort = opts.instancePort
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

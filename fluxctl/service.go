package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/weaveworks/flux/common/netutil"
	"github.com/weaveworks/flux/common/store"
)

const DEFAULT_RULE = "default"

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

func parseAddress(address string) (store.Service, error) {
	var svc store.Service
	if address == "" {
		return svc, nil
	}

	addr, err := netutil.ParseIPPort(address)
	if addr.IP() == nil {
		return svc, fmt.Errorf("expected IP address in '%s'", address)
	}

	svc.Address = &addr
	return svc, err
}

func (opts *addOpts) run(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("Expected argument <name>")
	}
	serviceName := args[0]

	svc, err := parseAddress(opts.address)
	if err != nil {
		return fmt.Errorf(`Did not understand the address supplied "%s"; expected to be ipaddress:port`,
			opts.address)
	}

	if opts.protocol != "" {
		svc.Protocol = opts.protocol
	}
	if opts.instancePort == 0 && svc.Address != nil {
		svc.InstancePort = svc.Address.Port()
	} else {
		svc.InstancePort = opts.instancePort
	}

	err = opts.store.AddService(serviceName, svc)
	if err != nil {
		return fmt.Errorf("Error updating service: %s", err)
	}

	spec, err := opts.makeSpec()
	if err != nil {
		return fmt.Errorf("Unable to extract rule from options: %s", err)
	}

	if spec != nil {
		if err = opts.store.SetContainerRule(serviceName, DEFAULT_RULE, *spec); err != nil {
			return fmt.Errorf("Error updating service: %s", err)
		}
	}

	fmt.Fprintln(opts.getStdout(), serviceName)
	return nil
}

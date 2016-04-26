package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/weaveworks/flux/common/store"
)

type infoOpts struct {
	baseOpts

	service string
}

func (opts *infoOpts) makeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info",
		Short: "display info on all services",
		Long:  "Display status information on all services, or the given service",
		RunE:  opts.run,
	}
	cmd.Flags().StringVarP(&opts.service, "service", "s", "", "display only this service")
	return cmd
}

func (opts *infoOpts) run(_ *cobra.Command, args []string) error {
	hosts, err := opts.store.GetHosts()
	if err != nil {
		return err
	}
	fmt.Fprint(opts.getStdout(), "HOSTS\n")
	for _, host := range hosts {
		fmt.Fprintln(opts.getStdout(), host.IP)
	}
	fmt.Fprint(opts.getStdout(), "\nSERVICES\n")

	qopts := store.QueryServiceOptions{
		WithInstances:      true,
		WithContainerRules: true,
	}
	svcs := make(map[string]*store.ServiceInfo)
	if opts.service != "" {
		svcs[opts.service], err = opts.store.GetService(opts.service, qopts)
	} else {
		svcs, err = opts.store.GetAllServices(qopts)
	}
	if err != nil {
		return err
	}

	for name, svc := range svcs {
		if err := printService(opts.getStdout(), name, svc); err != nil {
			return err
		}
	}
	return nil
}

func printService(out io.Writer, name string, svc *store.ServiceInfo) error {
	fmt.Fprintln(out, name)

	if svc.Address != nil {
		fmt.Fprintf(out, "  Address: %s\n", svc.Address)
	}
	if svc.InstancePort != 0 {
		fmt.Fprintf(out, "  Instance port: %d\n", svc.InstancePort)
	}
	if svc.Protocol != "" {
		fmt.Fprintf(out, "  Protocol: %s\n", svc.Protocol)
	}

	fmt.Fprint(out, "  RULES\n")
	for ruleName, rule := range svc.ContainerRules {
		selectBytes, err := json.Marshal(rule.Selector)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "    %s %s\n", ruleName, selectBytes)
	}
	fmt.Fprint(out, "  INSTANCES\n")
	for instName, inst := range svc.Instances {
		fmt.Fprintf(out, "    %s %s\n", instName, inst.Address)
	}
	return nil
}

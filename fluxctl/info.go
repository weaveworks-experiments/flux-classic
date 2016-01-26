package main

import (
	"encoding/json"
	"fmt"
	"io"
	//	"text/tabwriter"
	//	"text/template"

	"github.com/spf13/cobra"

	//	"github.com/squaremo/flux/common/data"
	"github.com/squaremo/flux/common/store"
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
	var (
		svcs []*store.ServiceInfo
		err  error
	)
	qopts := store.QueryServiceOptions{
		WithInstances:      true,
		WithContainerRules: true,
	}
	if opts.service != "" {
		var svc *store.ServiceInfo
		svc, err = opts.store.GetService(opts.service, qopts)
		svcs = []*store.ServiceInfo{svc}
	} else {
		svcs, err = opts.store.GetAllServices(qopts)
	}
	if err != nil {
		return err
	}

	for _, svc := range svcs {
		if err := printService(opts.getStdout(), svc); err != nil {
			return err
		}
	}
	return nil
}

func printService(out io.Writer, svc *store.ServiceInfo) error {
	fmt.Fprintf(out, "%s\n", svc.Name)
	fmt.Fprint(out, "  RULES\n")
	for _, rule := range svc.ContainerRules {
		selectBytes, err := json.Marshal(rule.Selector)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "    %s %s\n", rule.Name, selectBytes)
	}
	fmt.Fprint(out, "  INSTANCES\n")
	for _, inst := range svc.Instances {
		fmt.Fprintf(out, "    %s %s:%d\n", inst.Name, inst.Address, inst.Port)
	}
	return nil
}

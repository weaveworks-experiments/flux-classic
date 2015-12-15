package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/squaremo/ambergreen/common/store"
)

type selectOpts struct {
	store store.Store
	spec
}

func (opts *selectOpts) addCommandTo(top *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "select <service> <name> [options]",
		Short: "include instances in a service",
		Run:   opts.run,
	}
	opts.addSpecVars(cmd)
	top.AddCommand(cmd)
}

func (opts *selectOpts) run(_ *cobra.Command, args []string) {
	if len(args) < 2 {
		exitWithErrorf("You must supply <service> and <name>")
	}
	serviceName, name := args[0], args[1]

	// Check that the service exists
	_, err := opts.store.GetServiceDetails(serviceName)
	if err != nil {
		exitWithErrorf("Error fetching service: ", err)
	}

	spec, err := opts.makeSpec()
	if err != nil {
		exitWithErrorf("Unable to parse options into instance spec: ", err)
	}

	if err = opts.store.SetInstanceGroupSpec(serviceName, name, *spec); err != nil {
		exitWithErrorf("Error updating service: ", err)
	}

	fmt.Println("Selected instance group", name, "in service", serviceName)
}

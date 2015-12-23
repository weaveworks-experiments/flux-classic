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
		Use:   "select <service> <rule>",
		Short: "include containers in a service",
		Long:  "Select containers to be instances of <service>, giving the selection a name <rule> so it can be rescinded later, and the properties to match (via the flags).",
		Run:   opts.run,
	}
	opts.addSpecVars(cmd)
	top.AddCommand(cmd)
}

func (opts *selectOpts) run(_ *cobra.Command, args []string) {
	if len(args) != 2 {
		exitWithErrorf("You must supply <service> and <rule>")
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

	if err = opts.store.SetContainerGroupSpec(serviceName, name, *spec); err != nil {
		exitWithErrorf("Error updating service: ", err)
	}

	fmt.Println("Selected containers as ", name, "into service", serviceName)
}

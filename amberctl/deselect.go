package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/squaremo/ambergreen/common/backends"
	"github.com/squaremo/ambergreen/common/data"
)

type deselectOpts struct {
	backend *backends.Backend
}

func (opts *deselectOpts) addCommandTo(top *cobra.Command) {
	top.AddCommand(&cobra.Command{
		Use:   "deselect <service> <group>",
		Short: "deselect a group of instances from a service",
		Run:   opts.run,
	})
}

func (opts *deselectOpts) run(_ *cobra.Command, args []string) {
	if len(args) != 2 {
		exitWithErrorf("Expected <service> and <group>")
	}
	serviceName, group := args[0], args[1]
	service, err := opts.backend.GetServiceDetails(serviceName)
	if err != nil {
		exitWithErrorf("Unable to update service %s: %s", serviceName, err)
	}
	specs := service.InstanceSpecs
	if specs != nil {
		delete(specs, data.InstanceGroup(group))
	}
	fmt.Printf("Deselected group %s from service %s\n", group, serviceName)
}

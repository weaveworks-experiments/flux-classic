package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/squaremo/ambergreen/common/data"
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
	service, err := opts.store.GetServiceDetails(serviceName)
	if err != nil {
		exitWithErrorf("Error fetching service: ", err)
	}

	spec, err := opts.makeSpec()
	if err != nil {
		exitWithErrorf("Unable to parse options into instance spec: ", err)
	}

	addInstanceSpec(&service, data.InstanceGroup(name), spec)
	if err = opts.store.AddService(serviceName, service); err != nil {
		exitWithErrorf("Error updating service: ", err)
	}
	fmt.Println("Selected instance group", name, "in service", serviceName)
}

func addInstanceSpec(service *data.Service, name data.InstanceGroup, spec *data.InstanceSpec) {
	specs := service.InstanceSpecs
	if specs == nil {
		specs = make(map[data.InstanceGroup]data.InstanceSpec)
		service.InstanceSpecs = specs
	}
	specs[name] = *spec
}

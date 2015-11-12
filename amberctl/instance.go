package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/squaremo/ambergreen/pkg/backends"
	"github.com/squaremo/ambergreen/pkg/data"
)

func addInstanceCommands(top *cobra.Command, backend *backends.Backend) {
	sel := selectOpts{backend: backend}
	sel.addCommandTo(top)
}

type selectOpts struct {
	image    string
	protocol string
	fixed    int
	mapped   int

	backend *backends.Backend
}

func (opts *selectOpts) AddVars(cmd *cobra.Command) {
	cmd.Flags().StringVar(&opts.image, "docker-image", "", "enrol instances that use this image")
	cmd.Flags().StringVar(&opts.protocol, "protocol", "tcp", `the protocol to assume for connections to the service; either "http" or "tcp"`)
	cmd.Flags().IntVar(&opts.fixed, "fixed", 0, "Use a fixed port, and get the IP from docker inspect")
	cmd.Flags().IntVar(&opts.mapped, "mapped", 0, "Use the host address mapped to the port given")
}

func (opts *selectOpts) makeSpec() (*data.InstanceSpec, error) {
	var addrSpec data.AddressSpec

	if opts.image != "" {
		if opts.mapped > 0 && opts.fixed > 0 {
			return nil, fmt.Errorf("You cannot have both fixed and mapped port for default instance spec")
		}
		if opts.mapped > 0 {
			addrSpec = data.AddressSpec{Type: "mapped", Port: opts.mapped}
		} else if opts.fixed > 0 {
			addrSpec = data.AddressSpec{Type: "fixed", Port: opts.fixed}
		} else {
			return nil, fmt.Errorf("If you supply a selector, you must supply either --fixed or --mapped")
		}
		return &data.InstanceSpec{
			AddressSpec: addrSpec,
			Selector:    map[string]string{"image": opts.image},
		}, nil
	} else {
		return nil, nil
	}
}

func (opts *selectOpts) addCommandTo(top *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "select <name> [options]",
		Short: "include instances in a service",
		Run:   opts.run,
	}
	opts.AddVars(cmd)
	top.AddCommand(cmd)
}

func (opts *selectOpts) run(_ *cobra.Command, args []string) {
	if len(args) < 2 {
		exitWithErrorf("You must supply <service> and <name>")
	}
	serviceName, name := args[0], args[1]
	service, err := opts.backend.GetServiceDetails(serviceName)
	if err != nil {
		exitWithErrorf("Error fetching service: ", err)
	}

	spec, err := opts.makeSpec()
	if err != nil {
		exitWithErrorf("Unable to parse options into instance spec: ", err)
	}

	addInstanceSpec(&service, data.InstanceGroup(name), spec)
	if err = opts.backend.AddService(serviceName, service); err != nil {
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

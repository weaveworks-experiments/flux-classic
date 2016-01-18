package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

type selectOpts struct {
	baseOpts
	spec
}

func (opts *selectOpts) makeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "select <service> <rule>",
		Short: "include containers in a service",
		Long:  "Select containers to be instances of <service>, giving the selection a name <rule> so it can be rescinded later, and the properties to match (via the flags).",
		RunE:  opts.run,
	}
	opts.addSpecVars(cmd)
	return cmd
}

func (opts *selectOpts) run(_ *cobra.Command, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("You must supply <service> and <rule>")
	}
	serviceName, name := args[0], args[1]

	// Check that the service exists
	err := opts.store.CheckRegisteredService(serviceName)
	if err != nil {
		return fmt.Errorf("Error fetching service: %s", err)
	}

	spec, err := opts.makeSpec()
	if err != nil {
		return fmt.Errorf("Unable to parse options into rule: %s", err)
	}
	if spec == nil {
		return fmt.Errorf("Nothing will be selected by empty rule")
	}

	fmt.Printf("spec %s", spec)
	if err = opts.store.SetContainerRule(serviceName, name, *spec); err != nil {
		return fmt.Errorf("Error updating service: %s", err)
	}

	fmt.Println(name)
	return nil
}

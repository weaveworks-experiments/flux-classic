package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

type deselectOpts struct {
	baseOpts
}

func (opts *deselectOpts) makeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deselect <service> <rule>",
		Short: "remove a container selection rule from a service",
		Long:  "Remove container selection rule <rule> from <service>. Containers may still be selected by other rules.",
		RunE:  opts.run,
	}
	return cmd
}

func (opts *deselectOpts) run(_ *cobra.Command, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("Expected <service> and <rule>")
	}
	serviceName, rule := args[0], args[1]

	// Check that the service exists
	if err := opts.store.CheckRegisteredService(serviceName); err != nil {
		return fmt.Errorf("Error fetching service: %s", err)
	}

	if err := opts.store.RemoveContainerRule(serviceName, rule); err != nil {
		return fmt.Errorf("Unable to update service %s: %s", serviceName, err)
	}

	fmt.Fprintf(opts.getStdout(), rule)
	return nil
}

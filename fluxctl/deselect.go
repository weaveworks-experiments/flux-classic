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
		Use:   "deselect <service> [<rule name>]",
		Short: "remove a container selection rule from a service",
		Long:  `Remove container selection rule <rule name> from <service>. Containers may still be selected by other rules. If <rule name> is omitted, "default" is assumed`,
		RunE:  opts.run,
	}
	return cmd
}

func (opts *deselectOpts) run(_ *cobra.Command, args []string) error {
	var serviceName, ruleName string
	switch len(args) {
	case 1:
		ruleName = DEFAULT_RULE
	case 2:
		ruleName = args[1]
	default:
		return fmt.Errorf("Expected <service> and optionally, <rule name>")
	}
	serviceName = args[0]

	// Check that the service exists
	if err := opts.store.CheckRegisteredService(serviceName); err != nil {
		return fmt.Errorf("Error fetching service: %s", err)
	}

	if err := opts.store.RemoveContainerRule(serviceName, ruleName); err != nil {
		return fmt.Errorf("Unable to update service %s: %s", serviceName, err)
	}

	fmt.Fprintf(opts.getStdout(), ruleName)
	return nil
}

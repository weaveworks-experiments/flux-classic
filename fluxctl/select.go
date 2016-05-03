package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

type selectOpts struct {
	baseOpts
	spec

	instancePort int
}

func (opts *selectOpts) makeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "select <service> [<rule name>]",
		Short: "include containers in a service",
		Long:  `Select containers to be instances of <service>, giving the properties to match (via the flags). If <rule name> is omitted, "default" is assumed.`,
		RunE:  opts.run,
	}
	opts.addSpecVars(cmd)
	cmd.Flags().IntVar(&opts.instancePort, "instance-port", 0, "use this instance port instead of the default for the service")
	return cmd
}

func (opts *selectOpts) run(_ *cobra.Command, args []string) error {
	var serviceName, ruleName string

	switch len(args) {
	case 1:
		ruleName = DEFAULT_RULE
	case 2:
		ruleName = args[1]
	default:
		return fmt.Errorf("You must supply <service>, and you may supply <rule name>")
	}
	serviceName = args[0]

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

	if opts.instancePort != 0 {
		spec.InstancePort = opts.instancePort
	}

	if err = opts.store.SetContainerRule(serviceName, ruleName, *spec); err != nil {
		return fmt.Errorf("Error updating service: %s", err)
	}

	fmt.Fprintln(opts.getStdout(), ruleName)
	return nil
}

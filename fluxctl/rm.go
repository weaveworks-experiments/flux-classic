package main

import (
	"github.com/spf13/cobra"
)

type rmOpts struct {
	baseOpts
}

func (opts *rmOpts) makeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rm <service>|--all",
		Short: "remove service definition(s)",
		Long:  "Remove the service named <service>, or all services.",
		Run:   opts.run,
	}
	return cmd
}

func (opts *rmOpts) run(_ *cobra.Command, args []string) {
	var err error
	if len(args) != 1 {
		exitWithErrorf(`Please supply either a service name, or "--all"`)
	}
	if args[0] == "--all" {
		err = opts.store.RemoveAllServices()
	} else {
		err = opts.store.RemoveService(args[0])
	}
	if err != nil {
		exitWithErrorf("Failed to delete: " + err.Error())
	}
}

package main

import (
	"github.com/spf13/cobra"

	"github.com/squaremo/ambergreen/common/store"
)

type rmOpts struct {
	store store.Store
}

func (opts *rmOpts) addCommandTo(top *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "rm service|--all",
		Short: "remove service definition(s)",
		Long:  "Remove a single service, or all services.",
		Run:   opts.run,
	}
	top.AddCommand(cmd)
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

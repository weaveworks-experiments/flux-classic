package main

import (
	"github.com/spf13/cobra"

	"github.com/squaremo/ambergreen/common/store"
)

type rmOpts struct {
	store store.Store

	all bool
}

func (opts *rmOpts) addCommandTo(top *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "rm <service>|--all",
		Short: "remove service definition(s)",
		Run:   opts.run,
	}
	cmd.Flags().BoolVar(&opts.all, "all", false, "remove all service definitions")
	top.AddCommand(cmd)
}

func (opts *rmOpts) run(_ *cobra.Command, args []string) {
	var err error
	if opts.all {
		err = opts.store.RemoveAllServices()
	} else if len(args) == 1 {
		err = opts.store.RemoveService(args[0])
	} else {
		exitWithErrorf("Must supply service name or --all")
	}
	if err != nil {
		exitWithErrorf("Failed to delete: " + err.Error())
	}
}

package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/squaremo/ambergreen/common/backends"
	"github.com/squaremo/ambergreen/common/data"
)

type listOpts struct {
	backend *backends.Backend

	verbose bool
}

func (opts *listOpts) addCommandTo(top *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "list [options]",
		Short: "list the services defined",
		Run:   opts.run,
	}
	cmd.Flags().BoolVar(&opts.verbose, "verbose", false, "print the instances for each service in the list")
	top.AddCommand(cmd)
}

func (opts *listOpts) run(_ *cobra.Command, args []string) {
	printService := func(name string, value data.Service) { fmt.Println(name, value) }
	var printInstance func(name string, value data.Instance)
	if opts.verbose {
		printInstance = func(name string, value data.Instance) { fmt.Println("  ", name, value) }
	}
	err := opts.backend.ForeachServiceInstance(printService, printInstance)
	if err != nil {
		exitWithErrorf("Unable to enumerate services: ", err)
	}
}

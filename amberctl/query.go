package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/squaremo/ambergreen/common/backends"
	"github.com/squaremo/ambergreen/common/data"
)

type queryOpts struct {
	backend *backends.Backend

	service string
	short   bool
	selector
}

func (opts *queryOpts) addCommandTo(top *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "query [options]",
		Short: "display instances selected by the given filter",
		Run:   opts.run,
	}
	opts.addSelectorVars(cmd)
	cmd.Flags().StringVar(&opts.service, "service", "", "print only instances in service given")
	cmd.Flags().BoolVar(&opts.short, "short", false, "print only instance IDs (one per line)")
	top.AddCommand(cmd)
}

func printServiceHeader(name string, _ data.Service) {
	fmt.Println(name)
}

func printInstance(name string, inst data.Instance) {
	fmt.Println(name)
	fmt.Printf("%v\n", inst.Labels)
}

func (opts *queryOpts) run(_ *cobra.Command, args []string) {
	sel := opts.makeSelector()

	doService := printServiceHeader
	doInstance := func(name string, instance data.Instance) {
		if sel.Includes(instance) {
			if opts.short {
				fmt.Println(name)
			} else {
				printInstance(name, instance)
			}
		}
	}
	if opts.short {
		doService = func(_ string, _ data.Service) {}
	}

	if opts.service == "" {
		opts.backend.ForeachServiceInstance(doService, doInstance)
	} else {
		opts.backend.ForeachInstance(opts.service, doInstance)
	}
}

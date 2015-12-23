package main

import (
	"fmt"
	"os"
	"text/template"

	"github.com/spf13/cobra"

	"github.com/squaremo/ambergreen/common/data"
	"github.com/squaremo/ambergreen/common/store"
)

type queryOpts struct {
	store store.Store

	service string
	format  string
	selector
}

func (opts *queryOpts) addCommandTo(top *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "query",
		Short: "display instances selected by the given filter",
		Long:  "Display instances selected using the given filter, optionally for a single service only, and optionally formatting each result with a template rather than just printing the ID.",
		Run:   opts.run,
	}
	opts.addSelectorVars(cmd)
	cmd.Flags().StringVarP(&opts.service, "service", "s", "", "print only instances in <service>")
	cmd.Flags().StringVarP(&opts.format, "format", "f", "", "format each instance according to the go template given")
	top.AddCommand(cmd)
}

func printInstanceID(name string, inst data.Instance) {
	fmt.Println(name)
}

func (opts *queryOpts) run(_ *cobra.Command, args []string) {
	sel := opts.makeSelector()

	printInstance := printInstanceID

	var serviceName = opts.service

	if opts.format != "" {
		tmpl := template.Must(template.New("instance").Parse(opts.format))
		printInstance = func(name string, inst data.Instance) {
			err := tmpl.Execute(os.Stdout, instanceInfo{
				Service:  serviceName,
				Name:     name,
				Instance: inst,
			})
			if err != nil {
				panic(err)
			}
			fmt.Println()
		}
	}

	if opts.service == "" {
		store.SelectInstances(opts.store, sel, printInstance)
	} else {
		store.SelectServiceInstances(opts.store, opts.service, sel, printInstance)
	}
}

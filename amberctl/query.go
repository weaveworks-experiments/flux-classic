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
		Use:   "query [options]",
		Short: "display instances selected by the given filter",
		Run:   opts.run,
	}
	opts.addSelectorVars(cmd)
	cmd.Flags().StringVar(&opts.service, "service", "", "print only instances in <service>")
	cmd.Flags().StringVar(&opts.format, "format", "", "format each instance according to the go template given")
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

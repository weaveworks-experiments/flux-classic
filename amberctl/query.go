package main

import (
	"fmt"
	"os"
	"text/template"

	"github.com/spf13/cobra"

	"github.com/squaremo/ambergreen/common/backends"
	"github.com/squaremo/ambergreen/common/data"
)

type queryOpts struct {
	backend *backends.Backend

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

	doService := func(name string, _ data.Service) {
		serviceName = name
	}

	doInstance := func(name string, instance data.Instance) {
		if sel.Includes(instance) {
			printInstance(name, instance)
		}
	}

	if opts.service == "" {
		opts.backend.ForeachServiceInstance(doService, func(serviceName string, name string, inst data.Instance) {
			doInstance(name, inst)
		})
	} else {
		opts.backend.ForeachInstance(opts.service, doInstance)
	}
}

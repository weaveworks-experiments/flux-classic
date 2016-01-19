package main

import (
	"fmt"
	"text/template"

	"github.com/spf13/cobra"

	"github.com/squaremo/flux/common/data"
	"github.com/squaremo/flux/common/store"
)

type queryOpts struct {
	baseOpts
	selector

	service string
	format  string
}

func (opts *queryOpts) makeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "query",
		Short: "display instances selected by the given filter",
		Long:  "Display instances selected using the given filter, optionally for a single service only, and optionally formatting each result with a template rather than just printing the ID.",
		RunE:  opts.run,
	}
	opts.addSelectorVars(cmd)
	cmd.Flags().StringVarP(&opts.service, "service", "s", "", "print only instances in <service>")
	cmd.Flags().StringVarP(&opts.format, "format", "f", "", "format each instance according to the go template given")
	return cmd
}

type instanceForFormat struct {
	Service string `json:"service"`
	Name    string `json:"name"`
	data.Instance
}

func (opts *queryOpts) run(_ *cobra.Command, args []string) error {
	sel := opts.makeSelector()
	printInstance := func(_, name string, _ data.Instance) error {
		fmt.Fprintln(opts.getStdout(), name)
		return nil
	}

	if opts.format != "" {
		tmpl := template.Must(template.New("instance").Funcs(extraTemplateFuncs).Parse(opts.format))
		printInstance = func(serviceName, name string, inst data.Instance) error {
			err := tmpl.Execute(opts.getStdout(), instanceForFormat{
				Service:  serviceName,
				Name:     name,
				Instance: inst,
			})
			if err != nil {
				panic(err)
			}
			fmt.Fprintln(opts.getStdout())
			return nil
		}
	}

	if opts.service == "" {
		return store.SelectInstances(opts.store, sel, printInstance)
	} else {
		return store.SelectServiceInstances(opts.store, opts.service, sel, printInstance)
	}
}

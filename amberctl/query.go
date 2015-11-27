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
	short   bool
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
	cmd.Flags().StringVar(&opts.service, "service", "", "print only instances in service given")
	cmd.Flags().BoolVar(&opts.short, "short", false, "print only instance IDs (one per line)")
	cmd.Flags().StringVar(&opts.format, "format", "", "format each instance according to the go template given")
	top.AddCommand(cmd)
}

func printServiceHeader(name string, _ data.Service) {
	fmt.Println(name)
}

func printInstanceFull(name string, inst data.Instance) {
	fmt.Println(name)
	fmt.Printf("%v\n", inst)
}

type instanceInfo struct {
	Name    string
	Details data.Instance
}

func (opts *queryOpts) run(_ *cobra.Command, args []string) {
	sel := opts.makeSelector()

	doService := printServiceHeader
	printInstance := printInstanceFull

	if opts.format != "" {
		tmpl := template.Must(template.New("instance").Parse(opts.format))
		printInstance = func(name string, inst data.Instance) {
			err := tmpl.Execute(os.Stdout, instanceInfo{
				Name:    name,
				Details: inst,
			})
			if err != nil {
				panic(err)
			}
			fmt.Println()
		}
	}

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

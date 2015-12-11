package main

import (
	"fmt"
	"os"
	"text/template"

	"github.com/spf13/cobra"

	"github.com/squaremo/ambergreen/common/data"
	"github.com/squaremo/ambergreen/common/store"
)

type listOpts struct {
	store store.Store

	format         string
	formatInstance string
	verbose        bool
}

func (opts *listOpts) addCommandTo(top *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "list [options]",
		Short: "list the services defined",
		Run:   opts.run,
	}
	cmd.Flags().StringVar(&opts.format, "format", "", "format each service with the go template expression given")
	cmd.Flags().StringVar(&opts.formatInstance, "format-instance", "", "format each instance with the go template expression given (implies verbose)")
	cmd.Flags().BoolVar(&opts.verbose, "verbose", false, "show the list of instances for each service")
	top.AddCommand(cmd)
}

func (opts *listOpts) run(_ *cobra.Command, args []string) {
	printService := func(name string, _ data.Service) { fmt.Println(name) }
	if opts.format != "" {
		tmpl := template.Must(template.New("service").Parse(opts.format))
		printService = func(name string, serv data.Service) {
			var info serviceInfo
			info.Service = serv
			info.Name = name
			err := tmpl.Execute(os.Stdout, info)
			if err != nil {
				panic(err)
			}
			fmt.Println()
		}
	}

	var printInstance store.ServiceInstanceFunc

	if opts.verbose {
		printInstance = func(service, name string, value data.Instance) { fmt.Println("  ", name) }
	}
	if opts.formatInstance != "" {
		tmpl := template.Must(template.New("instance").Parse(opts.formatInstance))
		printInstance = func(name string, service string, inst data.Instance) {
			var info instanceInfo
			info.Instance = inst
			info.Name = name
			info.Service = service
			err := tmpl.Execute(os.Stdout, info)
			if err != nil {
				panic(err)
			}
			fmt.Println()
		}
	}

	err := opts.store.ForeachServiceInstance(printService, printInstance)
	if err != nil {
		exitWithErrorf("Unable to enumerate services: ", err)
	}
}

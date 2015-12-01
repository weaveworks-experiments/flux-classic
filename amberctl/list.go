package main

import (
	"fmt"
	"os"
	"text/template"

	"github.com/spf13/cobra"

	"github.com/squaremo/ambergreen/common/backends"
	"github.com/squaremo/ambergreen/common/data"
)

type listOpts struct {
	backend *backends.Backend

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
	printService := func(name string, value data.Service) { fmt.Println(name, value) }
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

	var printInstance func(name string, service string, value data.Instance)
	if opts.verbose {
		printInstance = func(name string, service string, value data.Instance) { fmt.Println("  ", name) }
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

	var serviceName string
	err := opts.backend.ForeachServiceInstance(func(name string, serv data.Service) {
		serviceName = name
		printService(name, serv)
	}, func(name string, inst data.Instance) {
		printInstance(name, serviceName, inst)
	})
	if err != nil {
		exitWithErrorf("Unable to enumerate services: ", err)
	}
}

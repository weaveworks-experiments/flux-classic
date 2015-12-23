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

	format     string
	formatRule string
	verbose    bool
}

func (opts *listOpts) addCommandTo(top *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "list the services defined",
		Long:  "List the services currently defined, optionally including the selection rules, and optionally formatting each result with a template rather than just printing the ID.",
		Run:   opts.run,
	}
	cmd.Flags().StringVarP(&opts.format, "format", "f", "", "format each service with the go template expression given")
	cmd.Flags().StringVar(&opts.formatRule, "format-rule", "", "format each rule with the go template expression given (implies --verbose)")
	cmd.Flags().BoolVarP(&opts.verbose, "verbose", "v", false, "show the list of selection rules for each service")
	top.AddCommand(cmd)
}

type ruleInfo struct {
	data.ContainerGroupSpec
	Name    string
	Service string
}

func (opts *listOpts) run(_ *cobra.Command, args []string) {
	printService := func(name string, _ data.Service) { fmt.Println(name) }
	if opts.format != "" {
		tmpl := template.Must(template.New("service").Funcs(extraTemplateFuncs).Parse(opts.format))
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

	if opts.formatRule != "" {
		opts.verbose = true
	} else {
		opts.formatRule = `  {{.Name}} {{json .Selector}}`
	}

	var printRule func(service, name string, rule data.ContainerGroupSpec)
	if opts.verbose {
		tmpl := template.Must(template.New("rule").Funcs(extraTemplateFuncs).Parse(opts.formatRule))
		printRule = func(serviceName, ruleName string, rule data.ContainerGroupSpec) {
			var info ruleInfo
			info.ContainerGroupSpec = rule
			info.Name = ruleName
			info.Service = serviceName
			err := tmpl.Execute(os.Stdout, info)
			if err != nil {
				panic(err)
			}
			fmt.Println()
		}
	}

	handleService := func(serviceName string, service data.Service) {
		printService(serviceName, service)
		if opts.verbose {
			rules, err := opts.store.GetContainerGroupSpecs(serviceName)
			if err != nil {
				panic(err)
			}
			for ruleName, rule := range rules {
				printRule(serviceName, ruleName, rule)
			}
		}
	}

	err := opts.store.ForeachServiceInstance(handleService, nil)
	if err != nil {
		exitWithErrorf("Unable to enumerate services: ", err)
	}
}

package main

import (
	"fmt"
	"os"
	"text/template"

	"github.com/spf13/cobra"

	"github.com/squaremo/flux/common/store"
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
	Service string
	*store.ContainerRuleInfo
}

func (opts *listOpts) run(_ *cobra.Command, args []string) {
	printService := func(s *store.ServiceInfo) error {
		fmt.Println(s.Name)
		return nil
	}
	if opts.format != "" {
		tmpl := template.Must(template.New("service").Funcs(extraTemplateFuncs).Parse(opts.format))
		printService = func(info *store.ServiceInfo) error {
			err := tmpl.Execute(os.Stdout, info)
			if err != nil {
				panic(err)
			}
			fmt.Println()
			return nil
		}
	}

	if opts.formatRule != "" {
		opts.verbose = true
	} else {
		opts.formatRule = `  {{.Name}} {{json .Selector}}`
	}

	var printRule func(serviceName string, rule *store.ContainerRuleInfo)
	if opts.verbose {
		tmpl := template.Must(template.New("rule").Funcs(extraTemplateFuncs).Parse(opts.formatRule))
		printRule = func(serviceName string, rule *store.ContainerRuleInfo) {
			var info ruleInfo
			info.ContainerRuleInfo = rule
			info.Service = serviceName
			err := tmpl.Execute(os.Stdout, info)
			if err != nil {
				panic(err)
			}
			fmt.Println()
		}
	}

	svcs, err := opts.store.GetAllServices(store.QueryServiceOptions{WithContainerRules: opts.verbose})
	if err != nil {
		exitWithErrorf("Unable to enumerate services: ", err)
	}
	for _, service := range svcs {
		printService(&service)
		if opts.verbose {
			rules := service.ContainerRules
			if err != nil {
				panic(err)
			}
			for _, rule := range rules {
				printRule(service.Name, &rule)
			}
		}
	}
}

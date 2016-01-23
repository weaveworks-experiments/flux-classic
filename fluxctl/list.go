package main

import (
	"fmt"
	"text/template"

	"github.com/spf13/cobra"

	"github.com/squaremo/flux/common/store"
)

type listOpts struct {
	baseOpts

	format     string
	formatRule string
	verbose    bool
}

func (opts *listOpts) makeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "list the services defined",
		Long:  "List the services currently defined, optionally including the selection rules, and optionally formatting each result with a template rather than just printing the ID.",
		RunE:  opts.run,
	}
	cmd.Flags().StringVarP(&opts.format, "format", "f", "", "format each service with the go template expression given")
	cmd.Flags().StringVar(&opts.formatRule, "format-rule", "", "format each rule with the go template expression given (implies --verbose)")
	cmd.Flags().BoolVarP(&opts.verbose, "verbose", "v", false, "show the list of selection rules for each service")
	return cmd
}

type ruleInfo struct {
	Service string `json:"service"`
	*store.ContainerRuleInfo
}

func (opts *listOpts) run(_ *cobra.Command, args []string) error {
	printService := func(s *store.ServiceInfo) error {
		fmt.Fprintln(opts.getStdout(), s.Name)
		return nil
	}
	if opts.format != "" {
		tmpl := template.Must(template.New("service").Funcs(extraTemplateFuncs).Parse(opts.format))
		printService = func(info *store.ServiceInfo) error {
			err := tmpl.Execute(opts.getStdout(), info)
			if err != nil {
				panic(err)
			}
			fmt.Fprintln(opts.getStdout())
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
			err := tmpl.Execute(opts.getStdout(), info)
			if err != nil {
				panic(err)
			}
			fmt.Fprintln(opts.getStdout())
		}
	}

	svcs, err := opts.store.GetAllServices(store.QueryServiceOptions{WithContainerRules: opts.verbose})
	if err != nil {
		return fmt.Errorf("Unable to enumerate services: ", err)
	}
	for _, service := range svcs {
		printService(service)
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
	return nil
}

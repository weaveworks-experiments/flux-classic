package main

import (
	"fmt"
	"text/tabwriter"
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
	quiet   bool
}

func (opts *queryOpts) makeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "query",
		Short: "display instances selected by the given filter",
		Long:  "Display instances selected using the given filter. By default the results are displayed in a table.",
		RunE:  opts.run,
	}
	opts.addSelectorVars(cmd)
	cmd.Flags().StringVarP(&opts.service, "service", "s", "", "print only instances in <service>")
	cmd.Flags().StringVarP(&opts.format, "format", "f", "", "format each instance according to the go template given (overrides --quiet)")
	cmd.Flags().BoolVarP(&opts.quiet, "quiet", "q", false, "print only instance names, one to a line")
	return cmd
}

type instanceForFormat struct {
	Service string `json:"service"`
	Name    string `json:"name"`
	data.Instance
}

const (
	tableHeaders     = "Service\tInstance name\tAddress\t\n"
	tableRowTemplate = "{{.Service}}\t{{.Name}}\t{{.Address}}:{{.Port}}\t\n"
)

func (opts *queryOpts) run(_ *cobra.Command, args []string) error {
	sel := opts.makeSelector()

	var printInstance func(string, string, data.Instance) error

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
	} else if opts.quiet {
		printInstance = func(_, name string, _ data.Instance) error {
			fmt.Fprintln(opts.getStdout(), name)
			return nil
		}
	} else {
		out := tabwriter.NewWriter(opts.getStdout(), 4, 0, 1, ' ', 0)
		defer out.Flush()
		tmpl := template.Must(template.New("row").Parse(tableRowTemplate))
		printInstance = func(serviceName, instanceName string, inst data.Instance) error {
			err := tmpl.Execute(out, instanceForFormat{
				Service:  serviceName,
				Name:     instanceName,
				Instance: inst,
			})
			if err != nil {
				return err
			}
			return nil
		}
		out.Write([]byte(tableHeaders))
	}

	if opts.service == "" {
		return store.SelectInstances(opts.store, sel, printInstance)
	} else {
		return store.SelectServiceInstances(opts.store, opts.service, sel, printInstance)
	}
}

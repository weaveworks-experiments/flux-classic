package main

import (
	"fmt"
	"text/tabwriter"
	"text/template"

	"github.com/spf13/cobra"

	"github.com/weaveworks/flux/common/data"
	"github.com/weaveworks/flux/common/store"
)

type queryOpts struct {
	baseOpts
	selector

	host    string
	state   string
	rule    string
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
	cmd.Flags().StringVar(&opts.host, "host", "", "select only containers on the given host")
	cmd.Flags().StringVar(&opts.state, "state", "", `select only containers in the given state (e.g., "live")`)
	cmd.Flags().StringVar(&opts.rule, "rule", "", "show only containers selected by the rule named")
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
	tableHeaders     = "SERVICE\tINSTANCE\tADDRESS\tSTATE\t\n"
	tableRowTemplate = "{{.Service}}\t{{.Name}}\t{{.Address}}:{{.Port}}\t{{.State}}"
)

func (opts *queryOpts) run(_ *cobra.Command, args []string) error {
	sel := opts.makeSelector()

	if opts.host != "" {
		sel[data.HostLabel] = opts.host
	}
	if opts.state != "" {
		sel[data.StateLabel] = opts.state
	}
	if opts.rule != "" {
		sel[data.RuleLabel] = opts.rule
	}

	var printInstance func(svc *store.ServiceInfo, inst *store.InstanceInfo) error
	if opts.quiet {
		printInstance = func(_ *store.ServiceInfo, inst *store.InstanceInfo) error {
			fmt.Fprintln(opts.getStdout(), inst.Name)
			return nil
		}
	} else {
		out := opts.getStdout()

		var tmpl *template.Template
		if opts.format == "" {
			tout := tabwriter.NewWriter(out, 4, 0, 2, ' ', 0)
			defer tout.Flush()
			tmpl = template.Must(template.New("row").Parse(tableRowTemplate))
			out = tout
			out.Write([]byte(tableHeaders))
		} else {
			tmpl = template.Must(template.New("instance").Funcs(extraTemplateFuncs).Parse(opts.format))
		}

		printInstance = func(svc *store.ServiceInfo, inst *store.InstanceInfo) error {
			err := tmpl.Execute(out, instanceForFormat{
				Service:  svc.Name,
				Name:     inst.Name,
				Instance: inst.Instance,
			})
			if err != nil {
				return err
			}
			fmt.Fprintln(out)
			return nil
		}
	}

	svcs := make([]*store.ServiceInfo, 1)
	var err error
	if opts.service == "" {
		svcs, err = opts.store.GetAllServices(store.QueryServiceOptions{WithInstances: true})
	} else {
		svcs[0], err = opts.store.GetService(opts.service, store.QueryServiceOptions{WithInstances: true})
	}

	if err != nil {
		return err
	}

	for _, svc := range svcs {
		for i := range svc.Instances {
			inst := &svc.Instances[i]
			if !sel.Includes(inst) {
				continue
			}

			if err := printInstance(svc, inst); err != nil {
				return err
			}
		}
	}

	return nil
}

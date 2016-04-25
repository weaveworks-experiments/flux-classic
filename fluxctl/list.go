package main

import (
	"fmt"
	"io"
	"text/template"

	"github.com/spf13/cobra"

	"github.com/weaveworks/flux/common/store"
)

type listOpts struct {
	baseOpts

	format     string
	formatRule string
	verbose    bool
}

const defaultFormat = "{{.Name}}"
const defaultVerboseFormat = `{{.Name}}{{if .Address}}
  Address: {{.Address}}{{end}}{{if (ne .InstancePort 0)}}
  Instance port: {{.InstancePort}}{{end}}{{if .Protocol}}
  Protocol: {{.Protocol}}{{end}}`

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
	Name    string `json:"name"`
	store.ContainerRule
}

func (opts *listOpts) run(_ *cobra.Command, args []string) error {
	format := opts.format
	if format == "" {
		format = defaultFormat
		if opts.verbose {
			format = defaultVerboseFormat
		}
	}

	tmpl := template.Must(template.New("service").Funcs(extraTemplateFuncs).Parse(format))

	if opts.formatRule != "" {
		opts.verbose = true
	} else {
		opts.formatRule = "  Rule: {{.Name}} {{json .Selector}}"
	}

	var ruleTmpl *template.Template
	if opts.verbose {
		ruleTmpl = template.Must(template.New("rule").Funcs(extraTemplateFuncs).Parse(opts.formatRule))
	}

	svcs, err := opts.store.GetAllServices(store.QueryServiceOptions{WithContainerRules: opts.verbose})
	if err != nil {
		return fmt.Errorf("Unable to enumerate services: %s", err)
	}
	for _, service := range svcs {
		err := executeTemplate(tmpl, opts.getStdout(), service)
		if err != nil {
			panic(err)
		}

		if ruleTmpl == nil {
			continue
		}

		for ruleName, rule := range service.ContainerRules {
			err := executeTemplate(ruleTmpl, opts.getStdout(), ruleInfo{
				Service:       service.Name,
				Name:          ruleName,
				ContainerRule: rule,
			})
			if err != nil {
				panic(err)
			}
		}
	}

	return nil
}

// Execute a template, making sure the result ends in a newline
func executeTemplate(tmpl *template.Template, wr io.Writer, data interface{}) error {
	enw := ensureNewlineWriter{wr, false}
	if err := tmpl.Execute(&enw, data); err != nil {
		return err
	}
	return enw.finish()
}

type ensureNewlineWriter struct {
	wr      io.Writer
	newline bool
}

func (enw *ensureNewlineWriter) Write(p []byte) (n int, err error) {
	if len(p) > 0 {
		enw.newline = (p[len(p)-1] == '\n')
	}
	return enw.wr.Write(p)
}

func (enw *ensureNewlineWriter) finish() (err error) {
	if !enw.newline {
		_, err = enw.wr.Write([]byte{'\n'})
	}
	return
}

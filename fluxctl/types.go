package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/squaremo/flux/common/data"
)

type selector struct {
	env    string
	labels string
	image  string
	tag    string
}

func selectorise(commaSeparatedLabels, keyPrefix string, intoSel map[string]string) {
	for _, kv := range strings.Split(commaSeparatedLabels, ",") {
		if kv == "" {
			continue
		}
		pair := strings.SplitN(strings.TrimLeft(kv, " "), "=", 2)
		switch len(pair) {
		case 0:
			continue
		case 1:
			intoSel[keyPrefix+pair[0]] = pair[0]
		case 2:
			intoSel[keyPrefix+pair[0]] = pair[1]
		}
	}
}

func (opts *selector) makeSelector() data.Selector {
	sel := make(map[string]string)
	selectorise(opts.labels, "", sel)
	selectorise(opts.env, "env.", sel)
	if opts.image != "" {
		sel["image"] = opts.image
	}
	if opts.tag != "" {
		sel["tag"] = opts.tag
	}
	return sel
}

func (opts *selector) addSelectorVars(cmd *cobra.Command) {
	cmd.Flags().StringVar(&opts.image, "image", "", "filter instances for this image")
	cmd.Flags().StringVar(&opts.tag, "tag", "", "filter instances for this tag")
	cmd.Flags().StringVar(&opts.labels, "labels", "", "filter instances for these labels, given as comma-delimited key=value pairs")
	cmd.Flags().StringVar(&opts.labels, "env", "", "filter instances for these environment variable values, given as comma-delimited key=value pairs")
}

type spec struct {
	fixed  int
	mapped int
	selector
}

func (opts *spec) addSpecVars(cmd *cobra.Command) {
	opts.addSelectorVars(cmd)
	cmd.Flags().IntVar(&opts.fixed, "port-fixed", 0, "Use a fixed port, and get the IP address from docker inspect")
	cmd.Flags().IntVar(&opts.mapped, "port-mapped", 0, "Use the host IP address, and the host port mapped to the given container port")
}

func (opts *spec) makeSpec() (*data.ContainerRule, error) {
	var addrSpec data.AddressSpec

	sel := opts.makeSelector()

	if !sel.Empty() {
		if opts.mapped > 0 && opts.fixed > 0 {
			return nil, fmt.Errorf("You cannot use both fixed and mapped port for a instance spec")
		}
		if opts.mapped > 0 {
			addrSpec = data.AddressSpec{Type: data.MAPPED, Port: opts.mapped}
		} else if opts.fixed > 0 {
			addrSpec = data.AddressSpec{Type: data.FIXED, Port: opts.fixed}
		} else {
			return nil, fmt.Errorf("Along with selection flags, you must supply one of --port-fixed or --port-mapped")
		}

		return &data.ContainerRule{
			AddressSpec: addrSpec,
			Selector:    sel,
		}, nil
	} else {
		return nil, nil
	}
}

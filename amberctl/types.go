package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/squaremo/ambergreen/common/data"
)

type selector struct {
	image string
	tag   string
}

func (opts *selector) makeSelector() data.Selector {
	sel := make(map[string]string)
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
}

type spec struct {
	protocol string
	fixed    int
	mapped   int
	selector
}

func (opts *spec) addSpecVars(cmd *cobra.Command) {
	opts.addSelectorVars(cmd)
	cmd.Flags().StringVar(&opts.protocol, "protocol", "tcp", `the protocol to assume for connections to the service; either "http" or "tcp"`)
	cmd.Flags().IntVar(&opts.fixed, "fixed", 0, "Use a fixed port, and get the IP from docker inspect")
	cmd.Flags().IntVar(&opts.mapped, "mapped", 0, "Use the host address mapped to the port given")
}

func (opts *spec) makeSpec() (*data.InstanceSpec, error) {
	var addrSpec data.AddressSpec

	sel := opts.makeSelector()

	if !sel.Empty() {
		if opts.mapped > 0 && opts.fixed > 0 {
			return nil, fmt.Errorf("You cannot have both fixed and mapped port for default instance spec")
		}
		if opts.mapped > 0 {
			addrSpec = data.AddressSpec{Type: "mapped", Port: opts.mapped}
		} else if opts.fixed > 0 {
			addrSpec = data.AddressSpec{Type: "fixed", Port: opts.fixed}
		} else {
			return nil, fmt.Errorf("If you supply a selector, you must supply either --fixed or --mapped")
		}

		return &data.InstanceSpec{
			AddressSpec: addrSpec,
			Selector:    sel,
		}, nil
	} else {
		return nil, nil
	}
}

// For formatted output

type serviceInfo struct {
	Name string
	data.Service
}

type instanceInfo struct {
	Service string
	Name    string
	data.Instance
}

package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/squaremo/ambergreen/common/backends"
)

type queryOpts struct {
	backend *backends.Backend

	service string
	selector
}

func (opts *queryOpts) addCommandTo(top *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "query [options]",
		Short: "query instances of a service",
		Run:   opts.run,
	}
	cmd.Flags().StringVar(&opts.image, "image", "", "query by docker image name")
	cmd.Flags().StringVar(&opts.tag, "tag", "", "query by docker image tag")
	top.AddCommand(cmd)
}

func (opts *queryOpts) run(_ *cobra.Command, args []string) {
	fmt.Printf("selected %+v\n", opts)
}

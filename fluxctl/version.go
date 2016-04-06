package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/weaveworks/flux/common/version"
)

type versionOpts struct {
	baseOpts
}

func (opts *versionOpts) makeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "print version and exit",
		RunE:  opts.run,
	}
}

func (opts *versionOpts) run(_ *cobra.Command, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("Unexpected arguments: %s", args)
	}
	fmt.Fprintln(opts.getStdout(), version.Banner())
	return nil
}

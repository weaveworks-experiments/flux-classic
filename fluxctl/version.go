package main

import (
	"fmt"
	"os"
	"path"

	"github.com/spf13/cobra"

	"github.com/squaremo/flux/common/version"
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
	fmt.Fprintf(opts.getStdout(), "%s version %s\n", path.Base(os.Args[0]), version.Version())
	return nil
}

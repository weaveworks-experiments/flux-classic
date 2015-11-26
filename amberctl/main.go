package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/squaremo/ambergreen/common/backends"
)

func main() {
	backend := backends.NewBackend([]string{})
	var topCmd = &cobra.Command{
		Use:   "amberctl",
		Short: "control ambergreen",
		Long:  `Create services and enrol instances in them`,
	}
	addSubCommands(topCmd, backend)
	if err := topCmd.Execute(); err != nil {
		exitWithErrorf(err.Error())
	}
}

type opts interface {
	addCommandTo(cmd *cobra.Command)
}

func addSubCommand(c opts, cmd *cobra.Command) {
	c.addCommandTo(cmd)
}

func addSubCommands(cmd *cobra.Command, backend *backends.Backend) {
	addSubCommand(&addOpts{backend: backend}, cmd)
	addSubCommand(&listOpts{backend: backend}, cmd)
	addSubCommand(&queryOpts{backend: backend}, cmd)
	addSubCommand(&rmOpts{backend: backend}, cmd)
	addSubCommand(&selectOpts{backend: backend}, cmd)
	addSubCommand(&deselectOpts{backend: backend}, cmd)
}

func exitWithErrorf(format string, vals ...interface{}) {
	fmt.Fprintf(os.Stderr, format, vals...)
	os.Exit(1)
}

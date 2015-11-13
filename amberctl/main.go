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
	addServiceCommands(topCmd, backend)
	addInstanceCommands(topCmd, backend)
	if err := topCmd.Execute(); err != nil {
		exitWithErrorf(err.Error())
	}
}

func exitWithErrorf(format string, vals ...interface{}) {
	fmt.Fprintf(os.Stderr, format, vals...)
	os.Exit(1)
}

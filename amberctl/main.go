package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/squaremo/ambergreen/common/store"
	"github.com/squaremo/ambergreen/common/store/etcdstore"
)

func main() {
	store := etcdstore.NewFromEnv()
	var topCmd = &cobra.Command{
		Use:   "amberctl",
		Short: "control ambergreen",
		Long:  `Create services and enrol instances in them`,
	}
	addSubCommands(topCmd, store)
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

func addSubCommands(cmd *cobra.Command, store store.Store) {
	addSubCommand(&addOpts{store: store}, cmd)
	addSubCommand(&listOpts{store: store}, cmd)
	addSubCommand(&queryOpts{store: store}, cmd)
	addSubCommand(&rmOpts{store: store}, cmd)
	addSubCommand(&selectOpts{store: store}, cmd)
	addSubCommand(&deselectOpts{store: store}, cmd)
}

func exitWithErrorf(format string, vals ...interface{}) {
	fmt.Fprintf(os.Stderr, format, vals...)
	os.Exit(1)
}

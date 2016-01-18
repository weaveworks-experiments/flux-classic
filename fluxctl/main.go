package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/squaremo/flux/common/store"
	"github.com/squaremo/flux/common/store/etcdstore"
)

func main() {
	store := etcdstore.NewFromEnv()
	var topCmd = &cobra.Command{
		Use:   "fluxctl",
		Short: "control flux",
		Long:  `Define services and enrol instances in them`,
	}
	addSubCommands(topCmd, store)
	if err := topCmd.Execute(); err != nil {
		exitWithErrorf(err.Error())
	}
}

func addSubCommand(c commandOpts, cmd *cobra.Command, st store.Store) {
	c.setStore(st)
	cmd.AddCommand(c.makeCommand())
}

func addSubCommands(cmd *cobra.Command, store store.Store) {
	addSubCommand(&addOpts{}, cmd, store)
	addSubCommand(&listOpts{}, cmd, store)
	addSubCommand(&queryOpts{}, cmd, store)
	addSubCommand(&rmOpts{}, cmd, store)
	addSubCommand(&selectOpts{}, cmd, store)
	addSubCommand(&deselectOpts{}, cmd, store)
}

func exitWithErrorf(format string, vals ...interface{}) {
	fmt.Fprintf(os.Stderr, format, vals...)
	os.Exit(1)
}

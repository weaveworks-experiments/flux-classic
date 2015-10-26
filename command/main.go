package main

import (
	"log"

	"github.com/spf13/cobra"

	"github.com/squaremo/ambergreen/pkg/backends"
)

var topCmd = &cobra.Command{
	Use:   "coatlctl",
	Short: "control weave Run",
	Long:  `Write more documentation here`,
}

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Commands to control services",
}

func setupCommands() {
	aso := &addServiceOpts{}
	add := addCommand(serviceCmd, "add <name> <address> <port>", "Register a new service", aso.addService)
	add.Flags().StringVar(&aso.dockerImage, "docker-image", "", "Docker image that implements this service")
	add.Flags().StringVar(&aso.protocol, "protocol", "", "Protocol, e.g. \"http\".  Defaults to simple tcp forwarding")

	lso1 := &listServiceOpts{}
	remove := addCommand(serviceCmd, "remove <name>|-a", "Clear out data for a service or all services", lso1.removeService)
	remove.Flags().BoolVarP(&lso1.all, "all", "a", false, "clear out all services")

	lso2 := &listServiceOpts{}
	list := addCommand(serviceCmd, "list", "List all registered services", lso2.listService)
	list.Flags().BoolVarP(&lso2.all, "all", "a", false, "list all information")
	topCmd.AddCommand(serviceCmd)

	addCommand(topCmd, "enrol <service> <instance> <address> <port>", "Enrol an instance in a service", enrol)
	addCommand(topCmd, "unenrol <service> <instance>", "Unenrol an instance from a service", unenrol)
}

func addCommand(parent *cobra.Command, use, short string, f func(args []string)) *cobra.Command {
	command := cobra.Command{
		Use:   use,
		Short: short,
		Run:   func(cmd *cobra.Command, args []string) { f(args) },
	}
	parent.AddCommand(&command)
	return &command
}

var backend *backends.Backend

func main() {
	backend = backends.NewBackend([]string{})
	setupCommands()

	err := topCmd.Execute()
	if err != nil {
		log.Fatal(err)
	}
}

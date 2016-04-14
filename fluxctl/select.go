package main

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type selectOpts struct {
	baseOpts
	spec
	name string
}

const RANDOM_NAME_SIZE_BYTES = 160 / 8

func randomName() string {
	bytes := make([]byte, RANDOM_NAME_SIZE_BYTES)
	rand.Read(bytes)
	return strings.ToLower(base32.HexEncoding.EncodeToString(bytes))
}

func (opts *selectOpts) makeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "select <service>",
		Short: "include containers in a service",
		Long:  "Select containers to be instances of <service>, giving the properties to match (via the flags).",
		RunE:  opts.run,
	}
	opts.addSpecVars(cmd)
	cmd.Flags().StringVar(&opts.name, "name", "", "give the selection a friendly name (otherwise it will get a random name)")
	return cmd
}

func (opts *selectOpts) run(_ *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("You must supply <service>")
	}
	serviceName := args[0]
	ruleName := opts.name
	if ruleName == "" {
		ruleName = randomName()
	}

	// Check that the service exists
	err := opts.store.CheckRegisteredService(serviceName)
	if err != nil {
		return fmt.Errorf("Error fetching service: %s", err)
	}

	spec, err := opts.makeSpec()
	if err != nil {
		return fmt.Errorf("Unable to parse options into rule: %s", err)
	}
	if spec == nil {
		return fmt.Errorf("Nothing will be selected by empty rule")
	}

	if err = opts.store.SetContainerRule(serviceName, ruleName, *spec); err != nil {
		return fmt.Errorf("Error updating service: %s", err)
	}

	fmt.Fprintln(opts.getStdout(), ruleName)
	return nil
}

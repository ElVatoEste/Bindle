// SPDX-License-Identifier: GPL-3.0-or-later

// Package cli wires the bindle command tree.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// version is set at build time via -ldflags "-X ...cli.version=...".
var version = "0.0.1-dev"

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "bindle",
		Short:         "Package & dependency manager for IBM i (ILE objects)",
		Long:          "Bindle declares, resolves, builds, and deploys reusable RPG/ILE business-logic modules.",
		SilenceUsage:  true,
		SilenceErrors: false,
		Version:       version,
	}

	root.AddCommand(
		newInitCmd(),
		newAddCmd(),
		newInstallCmd(),
		newBuildCmd(),
		newPublishCmd(),
		newListCmd(),
	)
	return root
}

// Execute runs the root command.
func Execute() error {
	return newRootCmd().Execute()
}

// notImplemented is a placeholder action for stubbed subcommands.
func notImplemented(name string) func(*cobra.Command, []string) error {
	return func(_ *cobra.Command, _ []string) error {
		return fmt.Errorf("%q not implemented yet", name)
	}
}
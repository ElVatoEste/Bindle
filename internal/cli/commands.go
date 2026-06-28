// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import "github.com/spf13/cobra"

func newAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <module>[@version]",
		Short: "Add a dependency to bindle.json",
		Args:  cobra.MinimumNArgs(1),
		RunE:  notImplemented("add"),
	}
}

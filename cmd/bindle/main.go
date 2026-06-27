// SPDX-License-Identifier: GPL-3.0-or-later

// Command bindle is the package & dependency manager for IBM i ILE objects.
package main

import (
	"os"

	"github.com/ElVatoEste/Bindle/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}

// SPDX-License-Identifier: GPL-3.0-or-later

// Package cli wires the bindle command tree.
package cli

import (
	"runtime/debug"

	"github.com/spf13/cobra"
)

// Build metadata, injected at release time via -ldflags (see .goreleaser.yaml).
// When not injected (e.g. `go install`), they fall back to Go's build info.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// resolveBuildInfo fills version/commit/date from the embedded build info when
// they weren't set by ldflags, so `go install ...@vX.Y.Z` shows a real version.
func resolveBuildInfo() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	if version == "dev" && info.Main.Version != "" && info.Main.Version != "(devel)" {
		version = info.Main.Version
	}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			if commit == "none" && s.Value != "" {
				commit = s.Value
				if len(commit) > 7 {
					commit = commit[:7]
				}
			}
		case "vcs.time":
			if date == "unknown" && s.Value != "" {
				date = s.Value
			}
		}
	}
}

// versionLine renders the version, appending "(commit, date)" only when known.
func versionLine() string {
	if commit == "none" && date == "unknown" {
		return "bindle " + version + "\n"
	}
	return "bindle " + version + " (" + commit + ", " + date + ")\n"
}

func newRootCmd() *cobra.Command {
	resolveBuildInfo()
	root := &cobra.Command{
		Use:           "bindle",
		Short:         "Package & dependency manager for IBM i (ILE objects)",
		Long:          "Bindle declares, resolves, builds, and deploys reusable RPG/ILE business-logic modules.",
		SilenceUsage:  true,
		SilenceErrors: false,
		Version:       version,
	}
	root.SetVersionTemplate(versionLine())

	root.AddCommand(
		newInitCmd(),
		newAddCmd(),
		newInstallCmd(),
		newBuildCmd(),
		newPublishCmd(),
		newListCmd(),
		newProfileCmd(),
		newPingCmd(),
		newExecCmd(),
		newPutCmd(),
		newGetCmd(),
		newSQLCmd(),
		newMigrateCmd(),
	)
	return root
}

// Execute runs the root command.
func Execute() error {
	return newRootCmd().Execute()
}

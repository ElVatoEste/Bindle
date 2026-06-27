// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/ElVatoEste/Bindle/internal/installer"
	"github.com/ElVatoEste/Bindle/internal/manifest"
	"github.com/ElVatoEste/Bindle/internal/registry"
)

const defaultCacheDir = ".bindle/cache"

func newInstallCmd() *cobra.Command {
	var file, regDir, cacheDir string
	var update bool

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Resolve, write the lock, and fetch artifacts to the local cache",
		Long: "Resolve dependencies (or reuse bindle.lock), write a reproducible lock, " +
			"then fetch and verify each artifact into the cache.\n\n" +
			"Note: deploy to an IBM i host (RSTLIB + migrations) is not implemented yet.",
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			return runInstall(c.OutOrStdout(), file, regDir, cacheDir, update)
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", manifest.FileName, "path to the manifest")
	cmd.Flags().StringVar(&regDir, "registry", "", "registry directory (overrides manifest registries.default)")
	cmd.Flags().StringVar(&cacheDir, "cache", defaultCacheDir, "directory for fetched artifacts")
	cmd.Flags().BoolVar(&update, "update", false, "re-resolve and rewrite the lock instead of reusing it")
	return cmd
}

func runInstall(w io.Writer, file, regDir, cacheDir string, update bool) error {
	m, err := manifest.Load(file)
	if err != nil {
		return err
	}
	root, err := registryDir(regDir, m)
	if err != nil {
		return err
	}

	res, err := installer.Install(registry.Open(root), installer.Options{
		ManifestPath: file,
		CacheDir:     cacheDir,
		Update:       update,
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(w, "%s %s\n", m.Name, m.Version)
	if res.LockWritten {
		fmt.Fprintf(w, "lock: %s (written)\n", res.LockPath)
	} else {
		fmt.Fprintf(w, "lock: %s (reused)\n", res.LockPath)
	}

	fmt.Fprintf(w, "fetched %d artifact(s) to %s:\n", len(res.Fetched), cacheDir)
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	for _, f := range res.Fetched {
		fmt.Fprintf(tw, "  %s\t%s\t%s\t(%d B)\n", f.Name, f.Version, f.Path, f.Bytes)
	}
	_ = tw.Flush()

	if len(res.Skipped) > 0 {
		fmt.Fprintf(w, "no artifact (skipped): %v\n", res.Skipped)
	}
	fmt.Fprintln(w, "note: deploy to IBM i (RSTLIB + migrations) not yet implemented")
	return nil
}

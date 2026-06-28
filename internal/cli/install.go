// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/ElVatoEste/Bindle/internal/config"
	"github.com/ElVatoEste/Bindle/internal/installer"
	"github.com/ElVatoEste/Bindle/internal/manifest"
	"github.com/ElVatoEste/Bindle/internal/registry"
	"github.com/ElVatoEste/Bindle/internal/transport"
)

const defaultCacheDir = ".bindle/cache"

func newInstallCmd() *cobra.Command {
	var file, regDir, cacheDir, lib string
	var update, deploy bool
	var ov config.Overrides
	var configPath string

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Resolve, write the lock, fetch artifacts, and optionally deploy to IBM i",
		Long: "Resolve dependencies (or reuse bindle.lock), write a reproducible lock, then " +
			"fetch and verify each artifact into the cache.\n\n" +
			"With --deploy, restore the artifacts onto the IBM i host (RSTOBJ), verify each " +
			"service program signature against the lock, and wire the library list.",
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			return runInstall(c.OutOrStdout(), installParams{
				file: file, regDir: regDir, cacheDir: cacheDir, update: update,
				deploy: deploy, lib: lib, configPath: configPath, ov: ov,
			})
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", manifest.FileName, "path to the manifest")
	cmd.Flags().StringVar(&regDir, "registry", "", "registry directory (overrides manifest registries.default)")
	cmd.Flags().StringVar(&cacheDir, "cache", defaultCacheDir, "directory for fetched artifacts")
	cmd.Flags().BoolVar(&update, "update", false, "re-resolve and rewrite the lock instead of reusing it")
	cmd.Flags().BoolVar(&deploy, "deploy", false, "restore artifacts onto the IBM i host")
	cmd.Flags().StringVar(&lib, "library", "", "override target library for deploy (e.g. on restricted hosts)")
	profileFlags(cmd, &ov, &configPath)
	return cmd
}

type installParams struct {
	file, regDir, cacheDir, lib, configPath string
	update, deploy                          bool
	ov                                      config.Overrides
}

func runInstall(w io.Writer, p installParams) error {
	m, err := manifest.Load(p.file)
	if err != nil {
		return err
	}
	root, err := registryDir(p.regDir, m)
	if err != nil {
		return err
	}

	res, err := installer.Install(registry.Open(root), installer.Options{
		ManifestPath: p.file,
		CacheDir:     p.cacheDir,
		Update:       p.update,
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

	fmt.Fprintf(w, "fetched %d artifact(s) to %s:\n", len(res.Fetched), p.cacheDir)
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	for _, f := range res.Fetched {
		fmt.Fprintf(tw, "  %s\t%s\t%s\t(%d B)\n", f.Name, f.Version, f.Path, f.Bytes)
	}
	_ = tw.Flush()
	if len(res.Skipped) > 0 {
		fmt.Fprintf(w, "no artifact (skipped): %v\n", res.Skipped)
	}

	if !p.deploy {
		fmt.Fprintln(w, "(local only — pass --deploy to install onto the IBM i host)")
		return nil
	}
	return deployToHost(w, p, res.LockPath)
}

func deployToHost(w io.Writer, p installParams, lockPath string) error {
	lock, err := manifest.LoadLock(lockPath)
	if err != nil {
		return err
	}
	prof, err := resolveProfile(p.configPath, p.ov)
	if err != nil {
		return err
	}
	conn, err := transport.DialSSH(*prof)
	if err != nil {
		return err
	}
	defer conn.Close()

	fmt.Fprintf(w, "deploying to %s@%s...\n", prof.User, prof.Host)
	deployed, err := installer.Deploy(conn, lock, installer.DeployOptions{
		CacheDir:        p.cacheDir,
		LibraryOverride: p.lib,
		WireLibList:     true,
		Logf:            func(format string, a ...any) { fmt.Fprintf(w, "  "+format+"\n", a...) },
	})
	if err != nil {
		return err
	}

	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	for _, d := range deployed {
		fmt.Fprintf(tw, "  installed\t%s %s\t%s/%s\t(sig %s)\n", d.Name, d.Version, d.Library, d.Srvpgm, shortSig(d.Signature))
	}
	_ = tw.Flush()
	fmt.Fprintf(w, "deployed %d package(s)\n", len(deployed))
	return nil
}

func shortSig(s string) string {
	if len(s) > 8 {
		return s[len(s)-8:]
	}
	return s
}

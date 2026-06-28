// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"fmt"
	"io"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/ElVatoEste/Bindle/internal/config"
	"github.com/ElVatoEste/Bindle/internal/installer"
	"github.com/ElVatoEste/Bindle/internal/manifest"
	"github.com/ElVatoEste/Bindle/internal/registry"
	"github.com/ElVatoEste/Bindle/internal/sqlchan"
	"github.com/ElVatoEste/Bindle/internal/transport"
	"github.com/ElVatoEste/Bindle/internal/ui"
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
	uo := ui.New(w)
	m, err := manifest.Load(p.file)
	if err != nil {
		return err
	}
	root, err := registryDir(p.regDir, m)
	if err != nil {
		return err
	}

	sp := uo.Spinner("resolving and fetching " + m.Name)
	sp.Start()
	res, err := installer.Install(registry.Open(root), installer.Options{
		ManifestPath: p.file,
		CacheDir:     p.cacheDir,
		Update:       p.update,
	})
	sp.Stop()
	if err != nil {
		uo.Fail("install failed")
		return err
	}

	uo.Heading("%s %s", m.Name, m.Version)
	lockState := "reused"
	if res.LockWritten {
		lockState = "written"
	}
	uo.KeyVal("lock", res.LockPath+" "+uo.Dim("("+lockState+")"))

	uo.OK("fetched %d artifact(s) to %s", len(res.Fetched), uo.Dim(p.cacheDir))
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	for _, f := range res.Fetched {
		fmt.Fprintf(tw, "    %s\t%s\t%s\n", uo.Bold(f.Name), f.Version, uo.Gray(fmt.Sprintf("%d B", f.Bytes)))
	}
	_ = tw.Flush()
	if len(res.Skipped) > 0 {
		uo.Bullet("no artifact (skipped): %v", res.Skipped)
	}

	if !p.deploy {
		uo.Info("%s", uo.Dim("(local only — pass --deploy to install onto the IBM i host)"))
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

	uo := ui.New(w)
	sp := uo.Spinner(fmt.Sprintf("deploying to %s@%s", prof.User, prof.Host))
	sp.Start()
	deployed, err := installer.Deploy(conn, lock, installer.DeployOptions{
		CacheDir:        p.cacheDir,
		LibraryOverride: p.lib,
		WireLibList:     true,
		Logf:            func(format string, a ...any) { sp.Update(format, a...) },
	})
	sp.Stop()
	if err != nil {
		uo.Fail("deploy failed")
		return err
	}

	for _, d := range deployed {
		uo.OK("installed %s %s %s %s", uo.Bold(d.Name), d.Version,
			uo.Dim(d.Library+"/"+d.Srvpgm), uo.Gray("sig "+shortSig(d.Signature)))
	}

	return runDeployMigrations(w, conn, lock, p.cacheDir)
}

// runDeployMigrations applies each deployed package's bundled migrations via the
// SQL channel, against the package's schema (lock schema, else its library).
func runDeployMigrations(w io.Writer, conn *transport.SSH, lock *manifest.Lock, cacheDir string) error {
	sql := sqlchan.NewDb2util(conn, "")
	for _, name := range sortedLockKeys(lock.Resolved) {
		e := lock.Resolved[name]
		if len(e.Migrations) == 0 {
			continue
		}
		schema := e.Schema
		if schema == "" {
			schema = e.Library
		}
		if schema == "" {
			return fmt.Errorf("%s: migrations present but no schema/library to apply them to", name)
		}
		dir := installer.MigrationsDir(cacheDir, name, e.Version)
		fmt.Fprintf(w, "migrating %s (schema %s)\n", name, schema)
		res, err := sqlchan.Migrate(sql, schema, dir, func(format string, a ...any) {
			fmt.Fprintf(w, "  "+format+"\n", a...)
		})
		if err != nil {
			return err
		}
		fmt.Fprintf(w, "  applied %d, skipped %d\n", len(res.Applied), len(res.Skipped))
	}
	return nil
}

func sortedLockKeys(m map[string]manifest.LockEntry) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func shortSig(s string) string {
	if len(s) > 8 {
		return s[len(s)-8:]
	}
	return s
}

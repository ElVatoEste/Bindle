// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ElVatoEste/Bindle/internal/config"
	"github.com/ElVatoEste/Bindle/internal/manifest"
	"github.com/ElVatoEste/Bindle/internal/sqlchan"
	"github.com/ElVatoEste/Bindle/internal/transport"
)

func newSQLCmd() *cobra.Command {
	var ov config.Overrides
	var configPath, db2util string

	cmd := &cobra.Command{
		Use:   "sql [flags] -- <statement>",
		Short: "Run a SQL statement on the IBM i host (via the SQL channel)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			return runSQL(c.OutOrStdout(), configPath, ov, db2util, strings.Join(args, " "))
		},
	}
	profileFlags(cmd, &ov, &configPath)
	cmd.Flags().StringVar(&db2util, "db2util", "", "path to db2util on the host (default "+sqlchan.DefaultDb2util+")")
	cmd.Flags().StringVar(&sqlBackend, "sql-backend", "db2util", "SQL backend: db2util | mapepire")
	return cmd
}

func runSQL(w io.Writer, configPath string, ov config.Overrides, db2util, stmt string) error {
	conn, closer, err := dialSQL(configPath, ov, db2util)
	if err != nil {
		return err
	}
	defer closer()

	if isQuery(stmt) {
		rows, err := conn.Query(stmt)
		if err != nil {
			return err
		}
		out, _ := json.MarshalIndent(rows, "", "  ")
		fmt.Fprintln(w, string(out))
		return nil
	}
	if err := conn.Exec(stmt); err != nil {
		return err
	}
	fmt.Fprintln(w, "ok")
	return nil
}

func newMigrateCmd() *cobra.Command {
	var ov config.Overrides
	var configPath, file, schema, db2util string

	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Apply a module's migrations to the IBM i host (via the SQL channel)",
		Long: "Run the module's migrations/ directory in order against the target schema, " +
			"tracked idempotently in a control table.",
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			return runMigrate(c.OutOrStdout(), configPath, ov, file, schema, db2util)
		},
	}
	profileFlags(cmd, &ov, &configPath)
	cmd.Flags().StringVarP(&file, "file", "f", manifest.FileName, "path to the manifest")
	cmd.Flags().StringVar(&schema, "schema", "", "target schema (default manifest migrations.schema or library)")
	cmd.Flags().StringVar(&db2util, "db2util", "", "path to db2util on the host")
	cmd.Flags().StringVar(&sqlBackend, "sql-backend", "db2util", "SQL backend: db2util | mapepire")
	return cmd
}

func runMigrate(w io.Writer, configPath string, ov config.Overrides, file, schema, db2util string) error {
	m, err := manifest.Load(file)
	if err != nil {
		return err
	}
	dir := "migrations"
	if m.Migrations != nil && m.Migrations.Dir != "" {
		dir = m.Migrations.Dir
	}
	dir = filepath.Join(filepath.Dir(file), dir)

	if schema == "" {
		if m.Migrations != nil && m.Migrations.Schema != "" {
			schema = m.Migrations.Schema
		} else {
			schema = m.Library
		}
	}
	if schema == "" {
		return fmt.Errorf("no target schema: set migrations.schema/library or pass --schema")
	}

	conn, closer, err := dialSQL(configPath, ov, db2util)
	if err != nil {
		return err
	}
	defer closer()

	fmt.Fprintf(w, "migrating %s (schema %s)\n", m.Name, schema)
	res, err := sqlchan.Migrate(conn, schema, dir, func(format string, a ...any) {
		fmt.Fprintf(w, "  "+format+"\n", a...)
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "applied %d, skipped %d (already applied)\n", len(res.Applied), len(res.Skipped))
	return nil
}

// sqlBackend selects the SQL channel implementation.
var sqlBackend string

// dialSQL opens an SSH connection and wraps it as a SQL channel. The backend is
// db2util (default; runs the PASE CLI over SSH) or mapepire (WebSocket to the
// daemon, reached through an SSH tunnel).
func dialSQL(configPath string, ov config.Overrides, db2util string) (sqlchan.Conn, func(), error) {
	prof, err := resolveProfile(configPath, ov)
	if err != nil {
		return nil, nil, err
	}
	ssh, err := transport.DialSSH(*prof)
	if err != nil {
		return nil, nil, err
	}

	switch sqlBackend {
	case "", "db2util":
		return sqlchan.NewDb2util(ssh, db2util), func() { _ = ssh.Close() }, nil
	case "mapepire":
		mp, err := sqlchan.DialMapepire(context.Background(), sqlchan.MapepireOptions{
			Tunnel:   ssh,
			User:     prof.User,
			Password: prof.Password,
			Insecure: true,
		})
		if err != nil {
			_ = ssh.Close()
			return nil, nil, err
		}
		return mp, func() { _ = mp.Close(); _ = ssh.Close() }, nil
	default:
		_ = ssh.Close()
		return nil, nil, fmt.Errorf("unknown sql backend %q (use db2util or mapepire)", sqlBackend)
	}
}

// isQuery reports whether a statement returns a result set.
func isQuery(stmt string) bool {
	t := strings.ToUpper(strings.TrimSpace(stmt))
	for _, p := range []string{"SELECT ", "VALUES ", "WITH "} {
		if strings.HasPrefix(t, p) {
			return true
		}
	}
	return false
}

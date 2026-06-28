// SPDX-License-Identifier: GPL-3.0-or-later

package sqlchan

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ControlTable is the per-schema table tracking applied migrations.
const ControlTable = "BINDLE_MIGRATIONS"

// schemaSetter is an optional Conn capability: point the connection at a schema
// so unqualified object names resolve there.
type schemaSetter interface {
	SetSchema(schema string) error
}

// Migration is one versioned DDL/DML file.
type Migration struct {
	ID       string // filename without extension, e.g. "0001_init"
	Path     string
	Checksum string // sha256 hex of the file contents
}

// MigrationResult records what Migrate did.
type MigrationResult struct {
	Applied []string // ids applied this run, in order
	Skipped []string // ids already applied
}

// ChecksumMismatchError means an already-applied migration's file changed —
// published migrations must be immutable.
type ChecksumMismatchError struct {
	ID, Was, Now string
}

func (e *ChecksumMismatchError) Error() string {
	return fmt.Sprintf("migration %s changed after being applied (was %s, now %s) — migrations are immutable",
		e.ID, short(e.Was), short(e.Now))
}

// LoadMigrations reads *.sql files from dir, sorted lexicographically by name.
func LoadMigrations(dir string) ([]Migration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}
	var migs []Migration
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".sql") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", e.Name(), err)
		}
		sum := sha256.Sum256(data)
		migs = append(migs, Migration{
			ID:       strings.TrimSuffix(e.Name(), filepath.Ext(e.Name())),
			Path:     path,
			Checksum: hex.EncodeToString(sum[:]),
		})
	}
	sort.Slice(migs, func(i, j int) bool { return migs[i].ID < migs[j].ID })
	return migs, nil
}

// Migrate applies pending migrations in order against the schema. It is
// idempotent (re-running applies nothing), ordered, and checksum-guarded.
func Migrate(conn Conn, schema, dir string, logf func(string, ...any)) (*MigrationResult, error) {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	migs, err := LoadMigrations(dir)
	if err != nil {
		return nil, err
	}
	// Point the connection at the target schema so unqualified objects in the
	// migration bodies resolve there (backends that need it, e.g. mapepire).
	if ss, ok := conn.(schemaSetter); ok {
		if err := ss.SetSchema(schema); err != nil {
			return nil, fmt.Errorf("set schema %s: %w", schema, err)
		}
	}
	if err := ensureControlTable(conn, schema); err != nil {
		return nil, err
	}
	applied, err := appliedChecksums(conn, schema)
	if err != nil {
		return nil, err
	}

	res := &MigrationResult{}
	for _, m := range migs {
		if was, ok := applied[m.ID]; ok {
			if !strings.EqualFold(was, m.Checksum) {
				return nil, &ChecksumMismatchError{ID: m.ID, Was: was, Now: m.Checksum}
			}
			res.Skipped = append(res.Skipped, m.ID)
			continue
		}
		logf("applying %s", m.ID)
		if err := applyMigration(conn, schema, m); err != nil {
			return nil, fmt.Errorf("migration %s: %w", m.ID, err)
		}
		res.Applied = append(res.Applied, m.ID)
	}
	return res, nil
}

func ensureControlTable(conn Conn, schema string) error {
	stmt := fmt.Sprintf(
		"CREATE TABLE %s.%s ("+
			"ID VARCHAR(128) NOT NULL PRIMARY KEY, "+
			"CHECKSUM CHAR(64) NOT NULL, "+
			"APPLIED_TS TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP)",
		schema, ControlTable)
	if err := conn.Exec(stmt); err != nil {
		// already exists is fine (SQL0601); other errors propagate
		if isAlreadyExists(err) {
			return nil
		}
		return err
	}
	return nil
}

func appliedChecksums(conn Conn, schema string) (map[string]string, error) {
	rows, err := conn.Query(fmt.Sprintf("SELECT ID, CHECKSUM FROM %s.%s", schema, ControlTable))
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(rows))
	for _, r := range rows {
		id := asString(r["ID"])
		out[id] = strings.TrimSpace(asString(r["CHECKSUM"]))
	}
	return out, nil
}

func applyMigration(conn Conn, schema string, m Migration) error {
	data, err := os.ReadFile(m.Path)
	if err != nil {
		return err
	}
	for _, stmt := range splitStatements(string(data)) {
		if err := conn.Exec(stmt); err != nil {
			return err
		}
	}
	// record it; ' in id unlikely but escape defensively
	rec := fmt.Sprintf("INSERT INTO %s.%s (ID, CHECKSUM) VALUES ('%s', '%s')",
		schema, ControlTable, sqlEscape(m.ID), m.Checksum)
	return conn.Exec(rec)
}

// splitStatements splits a SQL script on semicolons that terminate statements,
// ignoring semicolons inside single-quoted string literals. Comment-only and
// empty fragments are dropped.
func splitStatements(script string) []string {
	var stmts []string
	var b strings.Builder
	inStr := false
	for i := 0; i < len(script); i++ {
		c := script[i]
		switch {
		case c == '\'':
			inStr = !inStr
			b.WriteByte(c)
		case c == ';' && !inStr:
			if s := cleanStmt(b.String()); s != "" {
				stmts = append(stmts, s)
			}
			b.Reset()
		default:
			b.WriteByte(c)
		}
	}
	if s := cleanStmt(b.String()); s != "" {
		stmts = append(stmts, s)
	}
	return stmts
}

// cleanStmt trims a statement and strips leading line comments (-- ...).
func cleanStmt(s string) string {
	var keep []string
	for _, line := range strings.Split(s, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "--") {
			continue
		}
		keep = append(keep, line)
	}
	return strings.TrimSpace(strings.Join(keep, "\n"))
}

func isAlreadyExists(err error) bool {
	var se *SQLError
	if !errors.As(err, &se) {
		return false
	}
	d := se.Detail
	// SQL0601: name already exists; -601 SQLCODE
	return strings.Contains(d, "SQL0601") || strings.Contains(d, "-601") ||
		strings.Contains(strings.ToLower(d), "already exists")
}

func asString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", t)
	}
}

func sqlEscape(s string) string { return strings.ReplaceAll(s, "'", "''") }

func short(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}

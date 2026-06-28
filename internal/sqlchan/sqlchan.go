// SPDX-License-Identifier: GPL-3.0-or-later

// Package sqlchan is Bindle's SQL channel to an IBM i host: run SQL and read
// structured results, alongside the SSH/CL transport.
//
// The MVP backend drives the open-source db2util (PASE, installable via yum and
// present on pub400) over the existing SSH transport: it runs DDL/DML and returns
// query result sets as JSON. The Conn interface keeps room for a mapepire backend
// later without touching callers (e.g. migrations).
package sqlchan

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ElVatoEste/Bindle/internal/transport"
)

// Row is one result row: column name -> value.
type Row map[string]any

// Conn runs SQL against an IBM i host.
type Conn interface {
	// Exec runs a statement with no result set (DDL/DML).
	Exec(stmt string) error
	// Query runs a statement and returns its rows.
	Query(stmt string) ([]Row, error)
}

// Host is the slice of the SSH transport sqlchan needs.
type Host interface {
	Run(cmd string) (transport.Result, error)
}

// DefaultDb2util is the usual PASE path of db2util.
const DefaultDb2util = "/QOpenSys/pkgs/bin/db2util"

// Db2util is a Conn backed by the db2util CLI over SSH.
type Db2util struct {
	host Host
	bin  string
}

// NewDb2util returns a db2util-backed connection. An empty bin uses the default path.
func NewDb2util(h Host, bin string) *Db2util {
	if bin == "" {
		bin = DefaultDb2util
	}
	return &Db2util{host: h, bin: bin}
}

// Exec runs a statement and discards any output.
func (d *Db2util) Exec(stmt string) error {
	_, err := d.run(stmt, false)
	return err
}

// Query runs a statement and parses db2util's JSON result.
func (d *Db2util) Query(stmt string) ([]Row, error) {
	out, err := d.run(stmt, true)
	if err != nil {
		return nil, err
	}
	return parseRecords(out)
}

func (d *Db2util) run(stmt string, jsonOut bool) (string, error) {
	cmd := d.bin
	if jsonOut {
		cmd += " -o json"
	}
	cmd += " " + shellQuote(stmt)

	res, err := d.host.Run(cmd)
	if err != nil {
		return "", fmt.Errorf("sql transport: %w", err)
	}
	if res.Failed() {
		return "", &SQLError{Stmt: stmt, Detail: firstNonEmpty(res.Stderr, res.Stdout)}
	}
	// db2util reports SQL errors on stdout/stderr even with exit 0 in some builds;
	// surface them rather than parsing as data.
	if e := detectSQLError(res.Stdout + "\n" + res.Stderr); e != "" {
		return "", &SQLError{Stmt: stmt, Detail: e}
	}
	return res.Stdout, nil
}

// recordsDoc mirrors db2util's `-o json` shape: {"records":[ {...}, ... ]}.
type recordsDoc struct {
	Records []Row `json:"records"`
}

func parseRecords(out string) ([]Row, error) {
	out = strings.TrimSpace(out)
	if out == "" {
		return nil, nil
	}
	var doc recordsDoc
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		return nil, fmt.Errorf("parse db2util json: %w (output: %.200q)", err, out)
	}
	return doc.Records, nil
}

// SQLError is a statement that failed on the host.
type SQLError struct {
	Stmt   string
	Detail string
}

func (e *SQLError) Error() string {
	d := strings.TrimSpace(e.Detail)
	if d == "" {
		d = "(no detail)"
	}
	return fmt.Sprintf("sql failed: %s\n  statement: %s", d, oneLine(e.Stmt))
}

// detectSQLError looks for db2util's SQL failure markers in output.
func detectSQLError(s string) string {
	for _, line := range strings.Split(s, "\n") {
		t := strings.TrimSpace(line)
		// db2util emits SQLCODE/SQLSTATE and SQLxxxx messages on failure.
		if strings.Contains(t, "SQLSTATE") || strings.Contains(t, "SQLCODE") ||
			strings.HasPrefix(t, "SQL") && strings.Contains(t, "Error") {
			return t
		}
	}
	return ""
}

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

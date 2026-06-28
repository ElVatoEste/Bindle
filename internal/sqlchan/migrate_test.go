// SPDX-License-Identifier: GPL-3.0-or-later

package sqlchan

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// memConn is an in-memory Conn: it tracks a single control table.
type memConn struct {
	execed  []string
	applied map[string]string // id -> checksum (the control table)
	failOn  string            // substring of a stmt to fail on
}

func newMemConn() *memConn { return &memConn{applied: map[string]string{}} }

func (m *memConn) Exec(stmt string) error {
	m.execed = append(m.execed, stmt)
	if m.failOn != "" && strings.Contains(stmt, m.failOn) {
		return &SQLError{Stmt: stmt, Detail: "SQLSTATE=42000 forced"}
	}
	if strings.HasPrefix(stmt, "CREATE TABLE") && strings.Contains(stmt, ControlTable) {
		return &SQLError{Stmt: stmt, Detail: "SQL0601 already exists"} // simulate idempotent create
	}
	// crude INSERT into control table parser
	if strings.Contains(stmt, ControlTable) && strings.HasPrefix(stmt, "INSERT") {
		id, ck := parseInsert(stmt)
		m.applied[id] = ck
	}
	return nil
}

func (m *memConn) Query(stmt string) ([]Row, error) {
	rows := make([]Row, 0, len(m.applied))
	for id, ck := range m.applied {
		rows = append(rows, Row{"ID": id, "CHECKSUM": ck})
	}
	return rows, nil
}

func parseInsert(stmt string) (id, ck string) {
	i := strings.Index(stmt, "VALUES")
	rest := stmt[i:]
	parts := strings.Split(rest, "'")
	if len(parts) >= 4 {
		return parts[1], parts[3]
	}
	return "", ""
}

func writeMig(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateAppliesInOrderThenSkips(t *testing.T) {
	dir := t.TempDir()
	writeMig(t, dir, "0002_b.sql", "CREATE TABLE B (ID INT);")
	writeMig(t, dir, "0001_a.sql", "CREATE TABLE A (ID INT);")

	conn := newMemConn()
	res, err := Migrate(conn, "MYSCHEMA", dir, nil)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if len(res.Applied) != 2 || res.Applied[0] != "0001_a" || res.Applied[1] != "0002_b" {
		t.Fatalf("applied order wrong: %v", res.Applied)
	}

	// second run: all skipped, nothing applied
	res2, err := Migrate(conn, "MYSCHEMA", dir, nil)
	if err != nil {
		t.Fatalf("migrate 2: %v", err)
	}
	if len(res2.Applied) != 0 || len(res2.Skipped) != 2 {
		t.Errorf("second run: applied=%v skipped=%v (want 0/2)", res2.Applied, res2.Skipped)
	}
}

func TestMigrateChecksumMismatch(t *testing.T) {
	dir := t.TempDir()
	writeMig(t, dir, "0001_a.sql", "CREATE TABLE A (ID INT);")
	conn := newMemConn()
	if _, err := Migrate(conn, "S", dir, nil); err != nil {
		t.Fatal(err)
	}
	// change the file after it was applied
	writeMig(t, dir, "0001_a.sql", "CREATE TABLE A (ID INT, X INT);")
	_, err := Migrate(conn, "S", dir, nil)
	var mm *ChecksumMismatchError
	if !errors.As(err, &mm) {
		t.Fatalf("expected *ChecksumMismatchError, got %T: %v", err, err)
	}
}

func TestMigrateStopsOnError(t *testing.T) {
	dir := t.TempDir()
	writeMig(t, dir, "0001_a.sql", "CREATE TABLE A (ID INT);")
	writeMig(t, dir, "0002_bad.sql", "CREATE TABLE BADSYNTAX HERE;")
	conn := newMemConn()
	conn.failOn = "BADSYNTAX"
	res, err := Migrate(conn, "S", dir, nil)
	if err == nil {
		t.Fatal("expected error from bad migration")
	}
	if res != nil {
		t.Errorf("result should be nil on failure, got %+v", res)
	}
	// 0001 must have been applied before 0002 failed
	if conn.applied["0001_a"] == "" {
		t.Error("0001_a should have been applied before the failure")
	}
	if _, ok := conn.applied["0002_bad"]; ok {
		t.Error("0002_bad must not be recorded as applied")
	}
}

func TestLoadMigrationsSortsAndChecksums(t *testing.T) {
	dir := t.TempDir()
	writeMig(t, dir, "0002_b.sql", "x")
	writeMig(t, dir, "0001_a.sql", "y")
	writeMig(t, dir, "notes.txt", "ignored")
	migs, err := LoadMigrations(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(migs) != 2 || migs[0].ID != "0001_a" {
		t.Fatalf("migs = %+v", migs)
	}
	if migs[0].Checksum == "" || migs[0].Checksum == migs[1].Checksum {
		t.Error("checksums missing or not distinct")
	}
}

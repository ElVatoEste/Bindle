// SPDX-License-Identifier: GPL-3.0-or-later

package sqlchan

import (
	"strings"
	"testing"
)

func TestParseRecords(t *testing.T) {
	out := `{"records":[
{"ID":"0001_init","CHECKSUM":"abc"},
{"ID":"0002_more","CHECKSUM":"def"}
]}`
	rows, err := parseRecords(out)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if asString(rows[0]["ID"]) != "0001_init" || asString(rows[1]["CHECKSUM"]) != "def" {
		t.Errorf("rows wrong: %+v", rows)
	}
}

func TestParseRecordsEmpty(t *testing.T) {
	rows, err := parseRecords(`{"records":[]}`)
	if err != nil || len(rows) != 0 {
		t.Errorf("empty records = %v err=%v", rows, err)
	}
}

func TestSplitStatements(t *testing.T) {
	script := `-- create
CREATE TABLE T (ID INT);
INSERT INTO T VALUES (1);
-- a literal with a semicolon
INSERT INTO T2 VALUES ('a;b');
`
	got := splitStatements(script)
	if len(got) != 3 {
		t.Fatalf("got %d statements, want 3: %#v", len(got), got)
	}
	if !strings.Contains(got[2], "'a;b'") {
		t.Errorf("semicolon inside string literal was split: %q", got[2])
	}
	if strings.Contains(got[0], "--") {
		t.Errorf("leading comment not stripped: %q", got[0])
	}
}

func TestDetectSQLError(t *testing.T) {
	if detectSQLError("ok\nno errors here") != "" {
		t.Error("false positive")
	}
	if detectSQLError("SQLSTATE=42704 object not found") == "" {
		t.Error("missed SQLSTATE error")
	}
}

func TestIsAlreadyExists(t *testing.T) {
	if !isAlreadyExists(&SQLError{Detail: "SQL0601 name already exists"}) {
		t.Error("SQL0601 should be already-exists")
	}
	if isAlreadyExists(&SQLError{Detail: "SQL0204 not found"}) {
		t.Error("SQL0204 is not already-exists")
	}
}

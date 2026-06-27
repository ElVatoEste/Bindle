// SPDX-License-Identifier: GPL-3.0-or-later

package manifest

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Masterminds/semver/v3"
)

func TestLoadValidModule(t *testing.T) {
	m, err := Load(filepath.Join("testdata", "valid_module.json"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Name != "modfact" {
		t.Errorf("name = %q, want modfact", m.Name)
	}
	if m.Library != "MODFACT" {
		t.Errorf("library = %q, want MODFACT", m.Library)
	}
	if m.Exports == nil || m.Exports.Srvpgm != "FACTSRV" {
		t.Errorf("exports.srvpgm not parsed: %+v", m.Exports)
	}
	if !m.Publishable() {
		t.Error("module with exports should be publishable")
	}

	v, err := m.SemVer()
	if err != nil {
		t.Fatalf("SemVer: %v", err)
	}
	if v.Major() != 2 || v.Minor() != 3 {
		t.Errorf("version = %s, want 2.3.x", v)
	}

	c, err := m.Constraint("modbase")
	if err != nil {
		t.Fatalf("Constraint: %v", err)
	}
	ok, _ := c.Validate(mustVer(t, "1.5.0"))
	if !ok {
		t.Error("1.5.0 should satisfy modbase constraint >=1.0.0 <2.0.0")
	}
	if ok, _ := c.Validate(mustVer(t, "2.0.0")); ok {
		t.Error("2.0.0 should NOT satisfy >=1.0.0 <2.0.0")
	}
}

func TestLoadValidProject(t *testing.T) {
	m, err := Load(filepath.Join("testdata", "valid_project.json"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !m.Private {
		t.Error("project should be private")
	}
	if m.Publishable() {
		t.Error("private project must not be publishable")
	}
	if got := m.Registries["default"]; got != "ifs:///bindle/registry" {
		t.Errorf("registry = %q", got)
	}
}

func TestValidationErrors(t *testing.T) {
	cases := []struct {
		file string
		want string // substring expected in the error
	}{
		{"bad_library.json", "library"},
		{"bad_constraint.json", "constraint"},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			_, err := Load(filepath.Join("testdata", tc.file))
			if err == nil {
				t.Fatal("expected validation error, got nil")
			}
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("expected *ValidationError, got %T: %v", err, err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not mention %q", err.Error(), tc.want)
			}
		})
	}
}

func TestUnknownFieldRejected(t *testing.T) {
	_, err := Load(filepath.Join("testdata", "unknown_field.json"))
	if err == nil {
		t.Fatal("expected parse error for unknown field")
	}
	// must be a parse error, not a validation error
	var ve *ValidationError
	if errors.As(err, &ve) {
		t.Fatalf("unknown field should be a parse error, not ValidationError: %v", err)
	}
}

func TestValidateAggregatesIssues(t *testing.T) {
	raw := []byte(`{"schema":"wrong","name":"Bad_Name","version":"x.y","library":"toolonglibraryname"}`)
	_, err := Parse(raw)
	if err == nil {
		t.Fatal("expected validation error")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *ValidationError, got %T", err)
	}
	if len(ve.Issues) != 4 {
		t.Errorf("expected 4 issues (schema, name, version, library), got %d:\n%s",
			len(ve.Issues), err.Error())
	}
}

func mustVer(t *testing.T, s string) *semver.Version {
	t.Helper()
	v, err := semver.NewVersion(s)
	if err != nil {
		t.Fatalf("parse version %q: %v", s, err)
	}
	return v
}

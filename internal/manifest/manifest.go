// SPDX-License-Identifier: GPL-3.0-or-later

// Package manifest reads and validates bindle.json manifests and bindle.lock files.
//
// It is the source of truth for a module's identity, exports (service program,
// binder, signature, copy member), dependencies, build config, and migrations.
package manifest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"github.com/Masterminds/semver/v3"
)

// Schema identifiers for the on-disk formats.
const (
	SchemaV0     = "bindle/v0"
	LockSchemaV0 = "bindle-lock/v0"
)

// FileName is the conventional manifest filename in a module/project root.
const FileName = "bindle.json"

// Manifest is a parsed bindle.json.
type Manifest struct {
	Schema       string            `json:"schema"`
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Description  string            `json:"description,omitempty"`
	Author       string            `json:"author,omitempty"`
	License      string            `json:"license,omitempty"`
	Private      bool              `json:"private,omitempty"`
	Library      string            `json:"library"`
	Exports      *Exports          `json:"exports,omitempty"`
	Dependencies map[string]string `json:"dependencies,omitempty"`
	Build        *Build            `json:"build,omitempty"`
	Migrations   *Migrations       `json:"migrations,omitempty"`
	Runtime      *Runtime          `json:"runtime,omitempty"`
	Registries   map[string]string `json:"registries,omitempty"`
}

// Exports describes the module's public interface.
type Exports struct {
	Srvpgm    string `json:"srvpgm,omitempty"`
	Binder    string `json:"binder,omitempty"`
	Signature string `json:"signature,omitempty"`
	Copy      string `json:"copy,omitempty"`
}

// Build configures how the module's objects are compiled.
type Build struct {
	Engine  string   `json:"engine,omitempty"`
	Src     string   `json:"src,omitempty"`
	Objects []string `json:"objects,omitempty"`
}

// Migrations points at the module's versioned DDL.
type Migrations struct {
	Dir    string `json:"dir,omitempty"`
	Schema string `json:"schema,omitempty"`
}

// Runtime holds runtime-resolution hints.
type Runtime struct {
	LibraryList []string `json:"libraryList,omitempty"`
}

// Load reads and validates a manifest from path.
func Load(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest %q: %w", path, err)
	}
	m, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return m, nil
}

// Parse decodes and validates a manifest from raw JSON bytes.
// Unknown fields are rejected to catch typos early.
func Parse(data []byte) (*Manifest, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()

	var m Manifest
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

// SemVer returns the parsed semantic version of the manifest.
func (m *Manifest) SemVer() (*semver.Version, error) {
	return semver.StrictNewVersion(m.Version)
}

// Constraint returns the parsed version constraint for a dependency.
func (m *Manifest) Constraint(dep string) (*semver.Constraints, error) {
	raw, ok := m.Dependencies[dep]
	if !ok {
		return nil, fmt.Errorf("no such dependency %q", dep)
	}
	return semver.NewConstraint(raw)
}

// Publishable reports whether the manifest may be published to a registry.
func (m *Manifest) Publishable() bool {
	return !m.Private && m.Exports != nil && m.Exports.Srvpgm != ""
}
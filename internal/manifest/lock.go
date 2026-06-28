// SPDX-License-Identifier: GPL-3.0-or-later

package manifest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

// LockFileName is the conventional lock filename in a project root.
const LockFileName = "bindle.lock"

// Lock is the resolved, reproducible dependency set produced by `bindle install`.
type Lock struct {
	Schema   string               `json:"schema"`
	Resolved map[string]LockEntry `json:"resolved"`
}

// LockEntry pins a single dependency to an exact version, signature, and artifact.
type LockEntry struct {
	Version      string   `json:"version"`
	Library      string   `json:"library,omitempty"`
	Srvpgm       string   `json:"srvpgm,omitempty"`
	Signature    string   `json:"signature,omitempty"`
	Artifact     string   `json:"artifact,omitempty"`
	Hash         string   `json:"hash,omitempty"`
	Dependencies []string `json:"dependencies,omitempty"`
	Schema       string   `json:"schema,omitempty"`
	Migrations   []string `json:"migrations,omitempty"`
}

// NewLock returns an empty lock with the current schema.
func NewLock() *Lock {
	return &Lock{Schema: LockSchemaV0, Resolved: map[string]LockEntry{}}
}

// LoadLock reads and validates a lock file from path.
func LoadLock(path string) (*Lock, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read lock %q: %w", path, err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()

	var l Lock
	if err := dec.Decode(&l); err != nil {
		return nil, fmt.Errorf("parse lock %q: %w", path, err)
	}
	if l.Schema != LockSchemaV0 {
		return nil, fmt.Errorf("%s: lock schema must be %q (got %q)", path, LockSchemaV0, l.Schema)
	}
	if l.Resolved == nil {
		l.Resolved = map[string]LockEntry{}
	}
	return &l, nil
}

// Save writes the lock to path. Output is deterministic: encoding/json sorts map
// keys, so the same resolution always produces byte-identical output.
func (l *Lock) Save(path string) error {
	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return fmt.Errorf("encode lock: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write lock %q: %w", path, err)
	}
	return nil
}

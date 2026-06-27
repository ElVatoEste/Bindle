// SPDX-License-Identifier: GPL-3.0-or-later

package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/ElVatoEste/Bindle/internal/resolver"
)

// VersionsFile is the per-package index file name inside the registry.
const VersionsFile = "versions.json"

// File is a registry backed by a local directory tree:
//
//	<root>/<name>/versions.json
//
// It implements resolver.Registry. This is the MVP backend; the IFS/SAVF/S3
// backends will satisfy the same interface later.
type File struct {
	root string
}

// Open returns a File registry rooted at dir.
func Open(dir string) *File { return &File{root: dir} }

// versionsDoc mirrors the on-disk versions.json schema.
type versionsDoc struct {
	Name     string        `json:"name"`
	Versions []versionMeta `json:"versions"`
}

type versionMeta struct {
	Version      string            `json:"version"`
	Signature    string            `json:"signature,omitempty"`
	Artifact     string            `json:"artifact,omitempty"`
	Hash         string            `json:"hash,omitempty"`
	Dependencies map[string]string `json:"dependencies,omitempty"`
	Yanked       bool              `json:"yanked,omitempty"`
}

// Versions returns the available (non-yanked) versions of a package.
// A missing package file yields an empty slice, which the resolver reports as
// "not found".
func (f *File) Versions(name string) ([]resolver.Available, error) {
	path := filepath.Join(f.root, name, VersionsFile)
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read registry entry %q: %w", path, err)
	}

	var doc versionsDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse %q: %w", path, err)
	}

	out := make([]resolver.Available, 0, len(doc.Versions))
	for _, v := range doc.Versions {
		if v.Yanked {
			continue
		}
		out = append(out, resolver.Available{
			Version:      v.Version,
			Signature:    v.Signature,
			Artifact:     v.Artifact,
			Hash:         v.Hash,
			Dependencies: v.Dependencies,
		})
	}
	return out, nil
}

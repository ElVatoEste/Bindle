// SPDX-License-Identifier: GPL-3.0-or-later

package registry

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"

	"github.com/Masterminds/semver/v3"

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

// Fetch returns the raw bytes of an artifact identified by its
// registry-relative path (the "artifact" field of versions.json).
func (f *File) Fetch(artifact string) ([]byte, error) {
	if artifact == "" {
		return nil, fmt.Errorf("empty artifact path")
	}
	full := filepath.Join(f.root, filepath.FromSlash(artifact))
	data, err := os.ReadFile(full)
	if err != nil {
		return nil, fmt.Errorf("fetch artifact %q: %w", full, err)
	}
	return data, nil
}

// PublishInput describes one version being published.
type PublishInput struct {
	Name         string
	Version      string
	Signature    string
	Dependencies map[string]string
	ArtifactName string // base filename to store, e.g. "MODFACT.savf"
	Artifact     []byte
}

// AlreadyExistsError is returned when publishing a version that already exists
// and force was not requested.
type AlreadyExistsError struct{ Name, Version string }

func (e *AlreadyExistsError) Error() string {
	return fmt.Sprintf("%s@%s already published (use --force to overwrite)", e.Name, e.Version)
}

// Publish writes the artifact under <name>/<version>/ and upserts the package's
// versions.json. It returns the registry-relative artifact path and its sha256
// digest ("sha256:...").
func (f *File) Publish(in PublishInput, force bool) (artifactRel, hash string, err error) {
	if in.Name == "" || in.Version == "" || in.ArtifactName == "" {
		return "", "", fmt.Errorf("publish requires name, version, and artifact name")
	}

	artifactRel = path.Join(in.Name, in.Version, in.ArtifactName)
	full := filepath.Join(f.root, filepath.FromSlash(artifactRel))

	doc, err := f.readVersions(in.Name)
	if err != nil {
		return "", "", err
	}
	for _, v := range doc.Versions {
		if v.Version == in.Version && !force {
			return "", "", &AlreadyExistsError{Name: in.Name, Version: in.Version}
		}
	}

	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return "", "", fmt.Errorf("create artifact dir: %w", err)
	}
	if err := os.WriteFile(full, in.Artifact, 0o644); err != nil {
		return "", "", fmt.Errorf("write artifact: %w", err)
	}
	sum := sha256.Sum256(in.Artifact)
	hash = "sha256:" + hex.EncodeToString(sum[:])

	doc.Name = in.Name
	doc.upsert(versionMeta{
		Version:      in.Version,
		Signature:    in.Signature,
		Artifact:     artifactRel,
		Hash:         hash,
		Dependencies: in.Dependencies,
	})
	if err := f.writeVersions(doc); err != nil {
		return "", "", err
	}
	return artifactRel, hash, nil
}

func (f *File) readVersions(name string) (*versionsDoc, error) {
	p := filepath.Join(f.root, name, VersionsFile)
	data, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return &versionsDoc{Name: name}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", p, err)
	}
	var doc versionsDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse %q: %w", p, err)
	}
	return &doc, nil
}

func (f *File) writeVersions(doc *versionsDoc) error {
	doc.sortDesc()
	dir := filepath.Join(f.root, doc.Name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create package dir: %w", err)
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("encode versions: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, VersionsFile), append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write versions.json: %w", err)
	}
	return nil
}

func (d *versionsDoc) upsert(v versionMeta) {
	for i := range d.Versions {
		if d.Versions[i].Version == v.Version {
			d.Versions[i] = v
			return
		}
	}
	d.Versions = append(d.Versions, v)
}

// sortDesc orders versions newest-first for stable, readable output.
func (d *versionsDoc) sortDesc() {
	sort.Slice(d.Versions, func(i, j int) bool {
		vi, ei := semver.NewVersion(d.Versions[i].Version)
		vj, ej := semver.NewVersion(d.Versions[j].Version)
		if ei != nil || ej != nil {
			return d.Versions[i].Version > d.Versions[j].Version
		}
		return vi.GreaterThan(vj)
	})
}

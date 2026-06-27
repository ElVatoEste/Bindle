// SPDX-License-Identifier: GPL-3.0-or-later

package installer

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ElVatoEste/Bindle/internal/manifest"
	"github.com/ElVatoEste/Bindle/internal/resolver"
)

// Registry is what the installer needs from a registry backend: list versions
// (for resolution) and fetch artifact bytes.
type Registry interface {
	Versions(name string) ([]resolver.Available, error)
	Fetch(artifact string) ([]byte, error)
}

// Options configures a local install.
type Options struct {
	ManifestPath string // path to bindle.json
	CacheDir     string // where artifacts are written
	Update       bool   // force re-resolve and rewrite the lock
}

// Fetched records one artifact written to the cache.
type Fetched struct {
	Name    string
	Version string
	Path    string
	Bytes   int
}

// Result summarizes a local install.
type Result struct {
	LockPath    string
	LockWritten bool      // true if the lock was (re)generated this run
	Fetched     []Fetched // artifacts written to the cache
	Skipped     []string  // resolved packages that have no artifact to fetch
}

// HashMismatchError is returned when a fetched artifact's digest does not match
// the lock. Installs abort rather than deploy a tampered or corrupt artifact.
type HashMismatchError struct {
	Package, Version, Want, Got string
}

func (e *HashMismatchError) Error() string {
	return fmt.Sprintf("%s@%s: hash mismatch: want %s, got sha256:%s",
		e.Package, e.Version, e.Want, e.Got)
}

// Install performs the local half of an install: resolve (or reuse the lock),
// write bindle.lock, then fetch and verify each artifact into the cache.
//
// It does NOT yet deploy to an IBM i host (RSTLIB + migrations); that step will
// consume the verified artifacts produced here.
func Install(reg Registry, opts Options) (*Result, error) {
	m, err := manifest.Load(opts.ManifestPath)
	if err != nil {
		return nil, err
	}

	lockPath := filepath.Join(filepath.Dir(opts.ManifestPath), manifest.LockFileName)
	res := &Result{LockPath: lockPath}

	lock, err := resolveOrReuseLock(m, reg, lockPath, opts.Update)
	if err != nil {
		return nil, err
	}
	if lock.written {
		if err := lock.Lock.Save(lockPath); err != nil {
			return nil, err
		}
		res.LockWritten = true
	}

	for _, name := range sortedKeys(lock.Lock.Resolved) {
		entry := lock.Lock.Resolved[name]
		if entry.Artifact == "" {
			res.Skipped = append(res.Skipped, name)
			continue
		}

		data, err := reg.Fetch(entry.Artifact)
		if err != nil {
			return nil, fmt.Errorf("fetch %s@%s: %w", name, entry.Version, err)
		}
		if entry.Hash != "" {
			if err := verifyHash(name, entry.Version, data, entry.Hash); err != nil {
				return nil, err
			}
		}

		dst := filepath.Join(opts.CacheDir, name, entry.Version, filepath.Base(filepath.FromSlash(entry.Artifact)))
		if err := writeFile(dst, data); err != nil {
			return nil, err
		}
		res.Fetched = append(res.Fetched, Fetched{Name: name, Version: entry.Version, Path: dst, Bytes: len(data)})
	}
	return res, nil
}

type lockState struct {
	*manifest.Lock
	written bool
}

// resolveOrReuseLock returns the existing lock (for reproducibility) unless it
// is missing or Update was requested, in which case it resolves fresh.
func resolveOrReuseLock(m *manifest.Manifest, reg resolver.Registry, lockPath string, update bool) (*lockState, error) {
	if !update {
		if l, err := manifest.LoadLock(lockPath); err == nil {
			return &lockState{Lock: l}, nil
		}
	}
	r, err := resolver.Resolve(m, reg)
	if err != nil {
		return nil, err
	}
	return &lockState{Lock: r.Lock(), written: true}, nil
}

func verifyHash(name, version string, data []byte, want string) error {
	algo, hexWant, ok := strings.Cut(want, ":")
	if !ok || algo != "sha256" {
		return fmt.Errorf("%s@%s: unsupported hash %q (want sha256:...)", name, version, want)
	}
	sum := sha256.Sum256(data)
	got := hex.EncodeToString(sum[:])
	if !strings.EqualFold(got, hexWant) {
		return &HashMismatchError{Package: name, Version: version, Want: want, Got: got}
	}
	return nil
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write artifact %q: %w", path, err)
	}
	return nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// SPDX-License-Identifier: GPL-3.0-or-later

package manifest

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
)

var (
	// nameRe matches lowercase kebab-case package names, e.g. "modfact", "mod-clientes".
	nameRe = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)
	// libRe matches IBM i object names: ≤10 chars, start with letter/#/$/@.
	libRe = regexp.MustCompile(`^[A-Z#$@][A-Z0-9#$@_]{0,9}$`)
)

// ValidationError aggregates all problems found in a manifest.
type ValidationError struct {
	Issues []string
}

func (e *ValidationError) Error() string {
	return "invalid manifest:\n  - " + strings.Join(e.Issues, "\n  - ")
}

// Validate checks the manifest against the v0 rules, collecting every problem.
func (m *Manifest) Validate() error {
	v := &ValidationError{}
	add := func(format string, a ...any) { v.Issues = append(v.Issues, fmt.Sprintf(format, a...)) }

	if m.Schema != SchemaV0 {
		add("schema must be %q (got %q)", SchemaV0, m.Schema)
	}

	switch {
	case m.Name == "":
		add("name is required")
	case !nameRe.MatchString(m.Name):
		add("name %q must be lowercase kebab-case (e.g. \"modfact\")", m.Name)
	}

	switch {
	case m.Version == "":
		add("version is required")
	default:
		if _, err := semver.StrictNewVersion(m.Version); err != nil {
			add("version %q is not valid semver", m.Version)
		}
	}

	switch {
	case m.Library == "":
		add("library is required")
	case !libRe.MatchString(m.Library):
		add("library %q must be a valid IBM i object name (uppercase, <=10 chars, start with letter/#/$/@)", m.Library)
	}

	for _, dep := range sortedKeys(m.Dependencies) {
		constraint := m.Dependencies[dep]
		if !nameRe.MatchString(dep) {
			add("dependency name %q must be lowercase kebab-case", dep)
		}
		if constraint == "" {
			add("dependency %q has an empty version constraint", dep)
			continue
		}
		if _, err := semver.NewConstraint(constraint); err != nil {
			add("dependency %q has invalid version constraint %q", dep, constraint)
		}
	}

	if m.Build != nil && m.Build.Engine != "" {
		switch m.Build.Engine {
		case "bob", "native":
		default:
			add("build.engine %q must be \"bob\" or \"native\"", m.Build.Engine)
		}
	}

	if len(v.Issues) > 0 {
		return v
	}
	return nil
}

// sortedKeys returns the keys of a string-keyed map in deterministic order,
// so validation messages are stable across runs.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

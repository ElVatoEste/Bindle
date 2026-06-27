// Package resolver builds the dependency graph, resolves semver ranges to
// concrete versions, validates ILE signatures, detects conflicts, and produces
// a reproducible bindle.lock.
package resolver

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"

	"github.com/ElVatoEste/Bindle/internal/manifest"
)

// Registry provides the data the resolver needs about available packages.
type Registry interface {
	// Versions returns the available published versions of a package.
	// An empty result means the package is unknown to the registry.
	Versions(name string) ([]Available, error)
}

// Available describes one published version of a package.
type Available struct {
	Version      string
	Signature    string
	Artifact     string
	Hash         string
	Dependencies map[string]string // name -> version constraint
}

// Resolution is the outcome of resolving a manifest's dependency graph.
type Resolution struct {
	// Order is a topological order: dependencies appear before dependents.
	Order []string
	// Selected maps each package name to the concrete version chosen.
	Selected map[string]Available
}

// Lock converts a resolution into a reproducible manifest.Lock.
func (r *Resolution) Lock() *manifest.Lock {
	l := manifest.NewLock()
	for name, av := range r.Selected {
		l.Resolved[name] = manifest.LockEntry{
			Version:      av.Version,
			Signature:    av.Signature,
			Artifact:     av.Artifact,
			Hash:         av.Hash,
			Dependencies: sortedKeys(av.Dependencies),
		}
	}
	return l
}

type constraintSrc struct {
	c      *semver.Constraints
	raw    string
	source string
}

// Resolve walks the dependency graph reachable from root, selecting for each
// package the highest version satisfying every accumulated constraint.
func Resolve(root *manifest.Manifest, reg Registry) (*Resolution, error) {
	constraints := map[string][]constraintSrc{}
	selected := map[string]Available{}
	versionsCache := map[string][]Available{}

	getVersions := func(name string) ([]Available, error) {
		if v, ok := versionsCache[name]; ok {
			return v, nil
		}
		vs, err := reg.Versions(name)
		if err != nil {
			return nil, fmt.Errorf("registry lookup for %q: %w", name, err)
		}
		if len(vs) == 0 {
			return nil, &NotFoundError{Package: name}
		}
		versionsCache[name] = vs
		return vs, nil
	}

	type pending struct {
		name, constraint, source string
	}
	var queue []pending
	enqueue := func(deps map[string]string, source string) {
		for _, name := range sortedKeys(deps) {
			queue = append(queue, pending{name: name, constraint: deps[name], source: source})
		}
	}
	enqueue(root.Dependencies, root.Name)

	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]

		c, err := semver.NewConstraint(p.constraint)
		if err != nil {
			return nil, fmt.Errorf("%s: invalid constraint %q for %q", p.source, p.constraint, p.name)
		}
		constraints[p.name] = append(constraints[p.name], constraintSrc{c: c, raw: p.constraint, source: p.source})

		versions, err := getVersions(p.name)
		if err != nil {
			return nil, err
		}

		best, ok := pickBest(versions, constraints[p.name])
		if !ok {
			return nil, &ConflictError{Package: p.name, Constraints: constraints[p.name]}
		}

		if cur, ok := selected[p.name]; ok && cur.Version == best.Version {
			continue // already settled on this version
		}
		selected[p.name] = best
		enqueue(best.Dependencies, p.name) // (re)process the chosen version's deps
	}

	order, err := topoSort(selected)
	if err != nil {
		return nil, err
	}
	return &Resolution{Order: order, Selected: selected}, nil
}

// pickBest returns the highest version satisfying every constraint.
func pickBest(versions []Available, cs []constraintSrc) (Available, bool) {
	var (
		best  Available
		bestV *semver.Version
		found bool
	)
	for _, av := range versions {
		v, err := semver.NewVersion(av.Version)
		if err != nil {
			continue // ignore unparsable published versions
		}
		satisfies := true
		for _, c := range cs {
			if !c.c.Check(v) {
				satisfies = false
				break
			}
		}
		if !satisfies {
			continue
		}
		if !found || v.GreaterThan(bestV) {
			best, bestV, found = av, v, true
		}
	}
	return best, found
}

// topoSort returns selected packages with dependencies before dependents.
func topoSort(selected map[string]Available) ([]string, error) {
	indeg := make(map[string]int, len(selected))
	dependents := map[string][]string{}
	for name := range selected {
		indeg[name] = 0
	}
	for name, av := range selected {
		for dep := range av.Dependencies {
			if _, ok := selected[dep]; !ok {
				continue
			}
			indeg[name]++
			dependents[dep] = append(dependents[dep], name)
		}
	}

	var ready []string
	for name, d := range indeg {
		if d == 0 {
			ready = append(ready, name)
		}
	}
	sort.Strings(ready)

	var order []string
	for len(ready) > 0 {
		n := ready[0]
		ready = ready[1:]
		order = append(order, n)

		next := append([]string(nil), dependents[n]...)
		sort.Strings(next)
		for _, m := range next {
			indeg[m]--
			if indeg[m] == 0 {
				ready = append(ready, m)
			}
		}
		sort.Strings(ready)
	}

	if len(order) != len(selected) {
		var stuck []string
		for name, d := range indeg {
			if d > 0 {
				stuck = append(stuck, name)
			}
		}
		sort.Strings(stuck)
		return nil, &CycleError{Members: stuck}
	}
	return order, nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// NotFoundError is returned when a required package is unknown to the registry.
type NotFoundError struct{ Package string }

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("package %q not found in registry", e.Package)
}

// ConflictError is returned when no version satisfies all accumulated constraints.
type ConflictError struct {
	Package     string
	Constraints []constraintSrc
}

func (e *ConflictError) Error() string {
	parts := make([]string, len(e.Constraints))
	for i, c := range e.Constraints {
		parts[i] = fmt.Sprintf("%s (from %s)", c.raw, c.source)
	}
	return fmt.Sprintf("no version of %q satisfies all constraints: %s",
		e.Package, strings.Join(parts, ", "))
}

// CycleError is returned when the dependency graph contains a cycle.
type CycleError struct{ Members []string }

func (e *CycleError) Error() string {
	return "dependency cycle detected involving: " + strings.Join(e.Members, ", ")
}

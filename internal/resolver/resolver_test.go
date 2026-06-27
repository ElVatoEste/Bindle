// SPDX-License-Identifier: GPL-3.0-or-later

package resolver

import (
	"errors"
	"reflect"
	"testing"

	"github.com/ElVatoEste/Bindle/internal/manifest"
)

// mockRegistry is an in-memory registry: name -> available versions.
type mockRegistry map[string][]Available

func (m mockRegistry) Versions(name string) ([]Available, error) {
	return m[name], nil
}

func rootWith(deps map[string]string) *manifest.Manifest {
	return &manifest.Manifest{
		Schema:       manifest.SchemaV0,
		Name:         "miapp",
		Version:      "0.1.0",
		Library:      "MIAPP",
		Dependencies: deps,
	}
}

func TestResolveChain(t *testing.T) {
	reg := mockRegistry{
		"modfact": {{Version: "2.3.0", Signature: "F23", Dependencies: map[string]string{
			"modbase": ">=1.0.0 <2.0.0",
			"modimp":  "^1.2.0",
		}}},
		"modbase": {{Version: "1.4.2", Signature: "B14"}},
		"modimp":  {{Version: "1.2.5", Signature: "I12"}},
	}
	res, err := Resolve(rootWith(map[string]string{"modfact": "^2.3.0"}), reg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(res.Selected) != 3 {
		t.Fatalf("selected = %d, want 3", len(res.Selected))
	}
	if res.Selected["modbase"].Version != "1.4.2" {
		t.Errorf("modbase = %s", res.Selected["modbase"].Version)
	}
	// dependencies must precede their dependents
	assertBefore(t, res.Order, "modbase", "modfact")
	assertBefore(t, res.Order, "modimp", "modfact")
}

func TestResolvePicksHighestSatisfying(t *testing.T) {
	reg := mockRegistry{
		"modfact": {
			{Version: "2.3.0"},
			{Version: "2.3.5"},
			{Version: "2.4.0"},
			{Version: "3.0.0"}, // excluded by ^2.3.0
		},
	}
	res, err := Resolve(rootWith(map[string]string{"modfact": "^2.3.0"}), reg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got := res.Selected["modfact"].Version; got != "2.4.0" {
		t.Errorf("modfact = %s, want 2.4.0", got)
	}
}

func TestResolveDiamond(t *testing.T) {
	// a->{b,c}; b->d>=1.0.0; c->d<2.0.0; d in {1.0.0,1.5.0,2.0.0} => pick 1.5.0
	reg := mockRegistry{
		"b": {{Version: "1.0.0", Dependencies: map[string]string{"d": ">=1.0.0"}}},
		"c": {{Version: "1.0.0", Dependencies: map[string]string{"d": "<2.0.0"}}},
		"d": {{Version: "1.0.0"}, {Version: "1.5.0"}, {Version: "2.0.0"}},
	}
	res, err := Resolve(rootWith(map[string]string{"b": "^1.0.0", "c": "^1.0.0"}), reg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got := res.Selected["d"].Version; got != "1.5.0" {
		t.Errorf("d = %s, want 1.5.0 (highest satisfying >=1.0.0 and <2.0.0)", got)
	}
}

func TestResolveConflict(t *testing.T) {
	// b needs d>=2.0.0, c needs d<2.0.0 => no version satisfies both
	reg := mockRegistry{
		"b": {{Version: "1.0.0", Dependencies: map[string]string{"d": ">=2.0.0"}}},
		"c": {{Version: "1.0.0", Dependencies: map[string]string{"d": "<2.0.0"}}},
		"d": {{Version: "1.5.0"}, {Version: "2.0.0"}},
	}
	_, err := Resolve(rootWith(map[string]string{"b": "^1.0.0", "c": "^1.0.0"}), reg)
	var ce *ConflictError
	if !errors.As(err, &ce) {
		t.Fatalf("expected *ConflictError, got %T: %v", err, err)
	}
	if ce.Package != "d" {
		t.Errorf("conflict package = %q, want d", ce.Package)
	}
}

func TestResolveNotFound(t *testing.T) {
	reg := mockRegistry{} // empty
	_, err := Resolve(rootWith(map[string]string{"ghost": "^1.0.0"}), reg)
	var nf *NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("expected *NotFoundError, got %T: %v", err, err)
	}
}

func TestResolveCycle(t *testing.T) {
	reg := mockRegistry{
		"a": {{Version: "1.0.0", Dependencies: map[string]string{"b": "^1.0.0"}}},
		"b": {{Version: "1.0.0", Dependencies: map[string]string{"a": "^1.0.0"}}},
	}
	_, err := Resolve(rootWith(map[string]string{"a": "^1.0.0"}), reg)
	var cy *CycleError
	if !errors.As(err, &cy) {
		t.Fatalf("expected *CycleError, got %T: %v", err, err)
	}
}

func TestResolutionToLock(t *testing.T) {
	reg := mockRegistry{
		"modfact": {{Version: "2.3.0", Signature: "F23", Artifact: "a.savf", Hash: "sha256:x",
			Dependencies: map[string]string{"modbase": "^1.0.0"}}},
		"modbase": {{Version: "1.4.2", Signature: "B14"}},
	}
	res, err := Resolve(rootWith(map[string]string{"modfact": "^2.3.0"}), reg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	lock := res.Lock()
	if lock.Schema != manifest.LockSchemaV0 {
		t.Errorf("lock schema = %q", lock.Schema)
	}
	entry := lock.Resolved["modfact"]
	if entry.Version != "2.3.0" || entry.Hash != "sha256:x" {
		t.Errorf("modfact entry wrong: %+v", entry)
	}
	if !reflect.DeepEqual(entry.Dependencies, []string{"modbase"}) {
		t.Errorf("modfact deps = %v, want [modbase]", entry.Dependencies)
	}
}

func assertBefore(t *testing.T, order []string, first, second string) {
	t.Helper()
	fi, si := -1, -1
	for i, n := range order {
		if n == first {
			fi = i
		}
		if n == second {
			si = i
		}
	}
	if fi == -1 || si == -1 {
		t.Fatalf("order %v missing %q or %q", order, first, second)
	}
	if fi >= si {
		t.Errorf("expected %q before %q in order %v", first, second, order)
	}
}
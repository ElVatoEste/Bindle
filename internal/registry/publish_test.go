// SPDX-License-Identifier: GPL-3.0-or-later

package registry

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPublishWritesArtifactAndVersions(t *testing.T) {
	root := t.TempDir()
	reg := Open(root)
	blob := []byte("MODFACT-OBJECT")

	rel, hash, err := reg.Publish(PublishInput{
		Name:         "modfact",
		Version:      "2.3.0",
		Signature:    "A1B2C3",
		Dependencies: map[string]string{"modbase": "^1.0.0"},
		ArtifactName: "MODFACT.savf",
		Artifact:     blob,
	}, false)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	if rel != "modfact/2.3.0/MODFACT.savf" {
		t.Errorf("artifact rel = %q", rel)
	}
	sum := sha256.Sum256(blob)
	if want := "sha256:" + hex.EncodeToString(sum[:]); hash != want {
		t.Errorf("hash = %q, want %q", hash, want)
	}
	if _, err := os.Stat(filepath.Join(root, "modfact", "2.3.0", "MODFACT.savf")); err != nil {
		t.Errorf("artifact not written: %v", err)
	}

	got, err := reg.Versions("modfact")
	if err != nil || len(got) != 1 {
		t.Fatalf("Versions after publish: %d err=%v", len(got), err)
	}
	if got[0].Hash != hash || got[0].Dependencies["modbase"] != "^1.0.0" {
		t.Errorf("published metadata wrong: %+v", got[0])
	}
}

func TestPublishRefusesExistingWithoutForce(t *testing.T) {
	reg := Open(t.TempDir())
	in := PublishInput{Name: "m", Version: "1.0.0", ArtifactName: "M.savf", Artifact: []byte("x")}

	if _, _, err := reg.Publish(in, false); err != nil {
		t.Fatalf("first publish: %v", err)
	}
	_, _, err := reg.Publish(in, false)
	var ae *AlreadyExistsError
	if !errors.As(err, &ae) {
		t.Fatalf("expected *AlreadyExistsError, got %T: %v", err, err)
	}
	if _, _, err := reg.Publish(in, true); err != nil {
		t.Errorf("force publish should succeed: %v", err)
	}
}

func TestPublishKeepsOtherVersions(t *testing.T) {
	reg := Open(t.TempDir())
	mk := func(v string) PublishInput {
		return PublishInput{Name: "m", Version: v, ArtifactName: "M.savf", Artifact: []byte(v)}
	}
	if _, _, err := reg.Publish(mk("1.0.0"), false); err != nil {
		t.Fatal(err)
	}
	if _, _, err := reg.Publish(mk("1.1.0"), false); err != nil {
		t.Fatal(err)
	}
	got, _ := reg.Versions("m")
	if len(got) != 2 {
		t.Fatalf("want 2 versions, got %d", len(got))
	}
	// newest-first ordering
	if got[0].Version != "1.1.0" {
		t.Errorf("first = %q, want 1.1.0 (newest first)", got[0].Version)
	}
}

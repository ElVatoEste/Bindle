// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPublishThenList publishes a module, then resolves it from a consumer —
// proving publish writes a registry that install/list can consume.
func TestPublishThenList(t *testing.T) {
	dir := t.TempDir()
	regDir := filepath.Join(dir, "registry")

	// a publishable module manifest + an artifact
	modManifest := filepath.Join(dir, "modfact.json")
	writeFileT(t, modManifest, `{"schema":"bindle/v0","name":"modfact","version":"2.3.0",
	  "library":"MODFACT","exports":{"srvpgm":"FACTSRV","signature":"A1B2C3"}}`)
	artifact := filepath.Join(dir, "MODFACT.savf")
	writeFileT(t, artifact, "FACT-OBJECT")

	var pub bytes.Buffer
	if err := runPublish(&pub, modManifest, regDir, artifact, "", false); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if !strings.Contains(pub.String(), "published modfact@2.3.0") {
		t.Errorf("publish output:\n%s", pub.String())
	}

	// a consumer that depends on the just-published module
	app := filepath.Join(dir, "bindle.json")
	writeFileT(t, app, `{"schema":"bindle/v0","name":"miapp","version":"0.1.0","private":true,
	  "library":"MIAPP","dependencies":{"modfact":"^2.3.0"}}`)

	var lst bytes.Buffer
	if err := runList(&lst, app, regDir, false); err != nil {
		t.Fatalf("list after publish: %v", err)
	}
	if !strings.Contains(lst.String(), "modfact") || !strings.Contains(lst.String(), "2.3.0") {
		t.Errorf("consumer did not resolve published module:\n%s", lst.String())
	}
}

func TestPublishRefusesPrivate(t *testing.T) {
	dir := t.TempDir()
	app := filepath.Join(dir, "bindle.json")
	writeFileT(t, app, `{"schema":"bindle/v0","name":"miapp","version":"0.1.0","private":true,"library":"MIAPP"}`)
	artifact := filepath.Join(dir, "x.savf")
	writeFileT(t, artifact, "x")

	var buf bytes.Buffer
	if err := runPublish(&buf, app, filepath.Join(dir, "reg"), artifact, "", false); err == nil {
		t.Fatal("expected refusal to publish a private project")
	}
}

func writeFileT(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// SPDX-License-Identifier: GPL-3.0-or-later

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadMissingFileIsEmpty(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("missing config should not error: %v", err)
	}
	if len(c.Profiles) != 0 {
		t.Errorf("expected empty profiles, got %v", c.Profiles)
	}
}

func TestResolveProfileWithDefaults(t *testing.T) {
	path := writeConfig(t, `{
	  "defaultProfile": "pub400",
	  "profiles": {
	    "pub400": { "host": "pub400.com", "port": 2222, "user": "VATODEV" }
	  }
	}`)
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	p, err := c.Resolve(Overrides{})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if p.Host != "pub400.com" || p.Port != 2222 || p.User != "VATODEV" {
		t.Errorf("profile = %+v", p)
	}
	if p.Transport != "ssh" || p.Auth != "key" {
		t.Errorf("defaults not applied: %+v", p)
	}
}

func TestResolvePrecedenceFlagOverEnvOverFile(t *testing.T) {
	path := writeConfig(t, `{"profiles":{"p":{"host":"file-host","user":"FILEUSER","port":22}}}`)
	c, _ := Load(path)

	t.Setenv("BINDLE_HOST", "env-host")
	t.Setenv("BINDLE_USER", "ENVUSER")

	// flag overrides env; env overrides file
	p, err := c.Resolve(Overrides{Profile: "p", Host: "flag-host"})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if p.Host != "flag-host" {
		t.Errorf("host = %q, want flag-host (flag wins)", p.Host)
	}
	if p.User != "ENVUSER" {
		t.Errorf("user = %q, want ENVUSER (env over file)", p.User)
	}
}

func TestResolvePasswordEnvSetsAuth(t *testing.T) {
	c := &Config{Profiles: map[string]Profile{}}
	t.Setenv("BINDLE_HOST", "h")
	t.Setenv("BINDLE_USER", "u")
	t.Setenv("BINDLE_PASSWORD", "secret")

	p, err := c.Resolve(Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if p.Auth != "password" || p.Password != "secret" {
		t.Errorf("password auth not applied: %+v", p)
	}
}

func TestResolveMissingHostUserErrors(t *testing.T) {
	c := &Config{Profiles: map[string]Profile{}}
	if _, err := c.Resolve(Overrides{}); err == nil {
		t.Fatal("expected error when host/user missing")
	}
}

func TestResolveUnknownProfileErrors(t *testing.T) {
	c := &Config{Profiles: map[string]Profile{}}
	if _, err := c.Resolve(Overrides{Profile: "ghost"}); err == nil {
		t.Fatal("expected error for unknown profile")
	}
}

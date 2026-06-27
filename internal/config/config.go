// SPDX-License-Identifier: GPL-3.0-or-later

// Package config loads Bindle's host-agnostic connection profiles.
//
// Bindle is never tied to one IBM i host: the host is configuration. Profiles
// live OUTSIDE any repository (default ~/.bindle/config.json) so credentials are
// never committed. A resolved profile is assembled from, in increasing
// precedence: built-in defaults < config file profile < environment < flags.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
)

// DefaultPort is the SSH port assumed when a profile omits one.
const DefaultPort = 22

// Config is the on-disk ~/.bindle/config.json.
type Config struct {
	DefaultProfile string             `json:"defaultProfile,omitempty"`
	Profiles       map[string]Profile `json:"profiles,omitempty"`
}

// Profile describes how to reach one IBM i host.
//
// Password is supported for convenience (the file already lives in the user's
// home, never in a repo), but key or agent auth is preferred. It can always be
// supplied at runtime via BINDLE_PASSWORD instead of being stored.
type Profile struct {
	Host           string `json:"host,omitempty"`
	Port           int    `json:"port,omitempty"`
	User           string `json:"user,omitempty"`
	Transport      string `json:"transport,omitempty"` // "ssh" (default) | "odbc"
	Auth           string `json:"auth,omitempty"`      // "key" (default) | "agent" | "password"
	KeyFile        string `json:"keyFile,omitempty"`
	Password       string `json:"password,omitempty"`
	DefaultLibrary string `json:"defaultLibrary,omitempty"`
}

// Overrides are values supplied by command-line flags (highest precedence).
type Overrides struct {
	Profile   string
	Host      string
	User      string
	Port      int
	Transport string
	KeyFile   string
}

// DefaultPath returns ~/.bindle/config.json.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate home directory: %w", err)
	}
	return filepath.Join(home, ".bindle", "config.json"), nil
}

// Load reads a config file. A missing file is not an error: it yields an empty
// config, so Bindle works with env/flags alone.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return &Config{Profiles: map[string]Profile{}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse config %q: %w", path, err)
	}
	if c.Profiles == nil {
		c.Profiles = map[string]Profile{}
	}
	return &c, nil
}

// Resolve assembles the effective profile: defaults < file profile < env < flags.
// It returns an error if no host or user can be determined.
func (c *Config) Resolve(ov Overrides) (*Profile, error) {
	name := firstNonEmpty(ov.Profile, os.Getenv("BINDLE_PROFILE"), c.DefaultProfile)

	var p Profile
	if name != "" {
		base, ok := c.Profiles[name]
		if !ok {
			return nil, fmt.Errorf("profile %q not found in config", name)
		}
		p = base
	}

	applyEnv(&p)
	applyOverrides(&p, ov)
	applyDefaults(&p)

	var missing []string
	if p.Host == "" {
		missing = append(missing, "host")
	}
	if p.User == "" {
		missing = append(missing, "user")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("incomplete connection: missing %v — set a profile in %s, or pass --host/--user, or BINDLE_HOST/BINDLE_USER",
			missing, configHint())
	}
	return &p, nil
}

func applyEnv(p *Profile) {
	if v := os.Getenv("BINDLE_HOST"); v != "" {
		p.Host = v
	}
	if v := os.Getenv("BINDLE_USER"); v != "" {
		p.User = v
	}
	if v := os.Getenv("BINDLE_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			p.Port = n
		}
	}
	if v := os.Getenv("BINDLE_TRANSPORT"); v != "" {
		p.Transport = v
	}
	if v := os.Getenv("BINDLE_KEYFILE"); v != "" {
		p.KeyFile = v
	}
	if v := os.Getenv("BINDLE_PASSWORD"); v != "" {
		p.Password = v
		p.Auth = "password"
	}
}

func applyOverrides(p *Profile, ov Overrides) {
	if ov.Host != "" {
		p.Host = ov.Host
	}
	if ov.User != "" {
		p.User = ov.User
	}
	if ov.Port != 0 {
		p.Port = ov.Port
	}
	if ov.Transport != "" {
		p.Transport = ov.Transport
	}
	if ov.KeyFile != "" {
		p.KeyFile = ov.KeyFile
	}
}

func applyDefaults(p *Profile) {
	if p.Port == 0 {
		p.Port = DefaultPort
	}
	if p.Transport == "" {
		p.Transport = "ssh"
	}
	if p.Auth == "" {
		if p.Password != "" {
			p.Auth = "password"
		} else {
			p.Auth = "key"
		}
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func configHint() string {
	if p, err := DefaultPath(); err == nil {
		return p
	}
	return "~/.bindle/config.json"
}

// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/spf13/cobra"

	"github.com/ElVatoEste/Bindle/internal/manifest"
	"github.com/ElVatoEste/Bindle/internal/registry"
)

func newAddCmd() *cobra.Command {
	var file, regDir string

	cmd := &cobra.Command{
		Use:   "add <module>[@version] ...",
		Short: "Add one or more dependencies to bindle.json",
		Long: "Add dependencies to the manifest. Without a version, the highest published " +
			"version is resolved from the registry and pinned as a compatible (^) range.",
		Args: cobra.MinimumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			return runAdd(c.OutOrStdout(), file, regDir, args)
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", manifest.FileName, "path to the manifest")
	cmd.Flags().StringVar(&regDir, "registry", "", "registry directory (overrides manifest registries.default)")
	return cmd
}

func runAdd(w io.Writer, file, regDir string, specs []string) error {
	m, err := manifest.Load(file)
	if err != nil {
		return err
	}
	root, err := registryDir(regDir, m)
	if err != nil {
		return err
	}
	reg := registry.Open(root)

	if m.Dependencies == nil {
		m.Dependencies = map[string]string{}
	}

	for _, spec := range specs {
		name, constraint := splitSpec(spec)
		if name == "" {
			return fmt.Errorf("invalid dependency %q", spec)
		}

		if constraint == "" {
			latest, err := latestVersion(reg, name)
			if err != nil {
				return err
			}
			constraint = "^" + latest
		} else if _, err := semver.NewConstraint(constraint); err != nil {
			return fmt.Errorf("invalid version constraint %q for %q", constraint, name)
		}

		m.Dependencies[name] = constraint
		fmt.Fprintf(w, "added %s %s\n", name, constraint)
	}

	if err := m.Validate(); err != nil {
		return err
	}
	if err := writeManifest(file, m); err != nil {
		return err
	}
	fmt.Fprintf(w, "updated %s\n", file)
	return nil
}

// splitSpec parses "name" or "name@constraint".
func splitSpec(spec string) (name, constraint string) {
	if i := strings.LastIndex(spec, "@"); i > 0 {
		return spec[:i], spec[i+1:]
	}
	return spec, ""
}

// latestVersion returns the highest published version of a package.
func latestVersion(reg *registry.File, name string) (string, error) {
	versions, err := reg.Versions(name)
	if err != nil {
		return "", err
	}
	if len(versions) == 0 {
		return "", fmt.Errorf("package %q not found in registry", name)
	}
	var best *semver.Version
	var bestRaw string
	for _, v := range versions {
		sv, err := semver.NewVersion(v.Version)
		if err != nil {
			continue
		}
		if best == nil || sv.GreaterThan(best) {
			best, bestRaw = sv, v.Version
		}
	}
	if best == nil {
		return "", fmt.Errorf("no valid versions for %q", name)
	}
	return bestRaw, nil
}

func writeManifest(path string, m *manifest.Manifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("encode manifest: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write %q: %w", path, err)
	}
	return nil
}

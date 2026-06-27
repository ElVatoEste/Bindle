// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ElVatoEste/Bindle/internal/manifest"
)

func newInitCmd() *cobra.Command {
	var (
		file, name, library, description     string
		asModule, asProject, force, skeleton bool
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Scaffold a new module or project (bindle.json)",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			if asModule && asProject {
				return fmt.Errorf("choose either --module or --project, not both")
			}
			return runInit(c.OutOrStdout(), initOptions{
				file:        file,
				name:        name,
				library:     library,
				description: description,
				module:      asModule,
				force:       force,
				skeleton:    skeleton,
			})
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", manifest.FileName, "path to write the manifest")
	cmd.Flags().StringVar(&name, "name", "", "package name (default: current directory)")
	cmd.Flags().StringVar(&library, "library", "", "IBM i library (default: derived from name)")
	cmd.Flags().StringVar(&description, "description", "", "short description")
	cmd.Flags().BoolVar(&asModule, "module", false, "scaffold a publishable module")
	cmd.Flags().BoolVar(&asProject, "project", false, "scaffold a consumer project (default)")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing manifest")
	cmd.Flags().BoolVar(&skeleton, "skeleton", false, "also create src/, binder/, migrations/ (module only)")
	return cmd
}

type initOptions struct {
	file, name, library, description string
	module, force, skeleton          bool
}

func runInit(w io.Writer, opts initOptions) error {
	if !opts.force {
		if _, err := os.Stat(opts.file); err == nil {
			return fmt.Errorf("%s already exists (use --force to overwrite)", opts.file)
		}
	}

	name := opts.name
	if name == "" {
		abs, err := filepath.Abs(filepath.Dir(opts.file))
		if err != nil {
			return err
		}
		name = normalizeName(filepath.Base(abs))
	}

	library := opts.library
	if library == "" {
		library = deriveLibrary(name)
	}

	m := buildManifest(name, library, opts.description, opts.module)
	if err := m.Validate(); err != nil {
		return fmt.Errorf("generated manifest is invalid (try --name/--library): %w", err)
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(opts.file, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write %q: %w", opts.file, err)
	}

	kind := "project"
	if opts.module {
		kind = "module"
	}
	fmt.Fprintf(w, "created %s (%s) — name=%s library=%s\n", opts.file, kind, name, library)

	if opts.module && opts.skeleton {
		for _, dir := range []string{"src", "binder", "migrations"} {
			d := filepath.Join(filepath.Dir(opts.file), dir)
			if err := os.MkdirAll(d, 0o755); err != nil {
				return fmt.Errorf("create %q: %w", d, err)
			}
			fmt.Fprintf(w, "created %s/\n", dir)
		}
	}
	return nil
}

func buildManifest(name, library, description string, module bool) *manifest.Manifest {
	m := &manifest.Manifest{
		Schema:      manifest.SchemaV0,
		Name:        name,
		Version:     "0.1.0",
		Description: description,
		Library:     library,
	}
	if module {
		srvpgm := truncate(library, 7) + "SRV"
		m.Exports = &manifest.Exports{
			Srvpgm: srvpgm,
			Binder: "binder/" + srvpgm + ".bnd",
			Copy:   truncate(library, 8) + "PR",
		}
		m.Build = &manifest.Build{Engine: "bob", Src: "src/", Objects: []string{}}
		m.Migrations = &manifest.Migrations{Dir: "migrations/", Schema: library}
		m.Runtime = &manifest.Runtime{LibraryList: []string{library}}
		return m
	}
	m.Private = true
	m.Dependencies = map[string]string{}
	m.Registries = map[string]string{"default": "ifs:///bindle/registry"}
	return m
}

var nonName = regexp.MustCompile(`[^a-z0-9-]+`)

// normalizeName lowercases and kebab-cases an arbitrary directory name.
func normalizeName(s string) string {
	s = strings.ToLower(s)
	s = nonName.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "app"
	}
	return s
}

// deriveLibrary turns a package name into a valid IBM i object name.
func deriveLibrary(name string) string {
	up := strings.ToUpper(name)
	var b strings.Builder
	for _, r := range up {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '#', r == '$', r == '@', r == '_':
			b.WriteRune(r)
		}
	}
	lib := b.String()
	if lib == "" || !(lib[0] >= 'A' && lib[0] <= 'Z' || lib[0] == '#' || lib[0] == '$' || lib[0] == '@') {
		lib = "LIB" + lib
	}
	return truncate(lib, 10)
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

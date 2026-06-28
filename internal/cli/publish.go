// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/ElVatoEste/Bindle/internal/manifest"
	"github.com/ElVatoEste/Bindle/internal/registry"
	"github.com/ElVatoEste/Bindle/internal/sqlchan"
)

func newPublishCmd() *cobra.Command {
	var file, regDir, artifact, signature, lib string
	var force bool

	cmd := &cobra.Command{
		Use:   "publish",
		Short: "Publish a module's artifact and metadata to the registry",
		Long: "Write a module's artifact under <name>/<version>/ in the registry and " +
			"upsert its versions.json (library, srvpgm, signature, hash, dependencies).",
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			return runPublish(c.OutOrStdout(), file, regDir, artifact, signature, lib, force)
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", manifest.FileName, "path to the manifest")
	cmd.Flags().StringVar(&regDir, "registry", "", "registry directory (overrides manifest registries.default)")
	cmd.Flags().StringVar(&artifact, "artifact", "", "path to the artifact (SAVF) to publish")
	cmd.Flags().StringVar(&signature, "signature", "", "ILE signature (defaults to exports.signature)")
	cmd.Flags().StringVar(&lib, "library", "", "library the artifact restores into (defaults to manifest.library)")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an already-published version")
	return cmd
}

func runPublish(w io.Writer, file, regDir, artifactPath, signature, lib string, force bool) error {
	m, err := manifest.Load(file)
	if err != nil {
		return err
	}
	if !m.Publishable() {
		return fmt.Errorf("%s is not publishable: it must be non-private and declare exports.srvpgm", m.Name)
	}
	if artifactPath == "" {
		return fmt.Errorf("--artifact is required (path to the SAVF to publish)")
	}

	root, err := registryDir(regDir, m)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(artifactPath)
	if err != nil {
		return fmt.Errorf("read artifact: %w", err)
	}

	if signature == "" && m.Exports != nil {
		signature = m.Exports.Signature
	}
	if lib == "" {
		lib = m.Library
	}

	schema, migs, err := collectMigrations(file, m, lib)
	if err != nil {
		return err
	}

	rel, hash, err := registry.Open(root).Publish(registry.PublishInput{
		Name:         m.Name,
		Version:      m.Version,
		Library:      lib,
		Srvpgm:       m.Exports.Srvpgm,
		Signature:    signature,
		Dependencies: m.Dependencies,
		ArtifactName: filepath.Base(artifactPath),
		Artifact:     data,
		Schema:       schema,
		Migrations:   migs,
	}, force)
	if err != nil {
		return err
	}

	fmt.Fprintf(w, "published %s@%s to %s\n", m.Name, m.Version, root)
	fmt.Fprintf(w, "  artifact: %s\n", rel)
	fmt.Fprintf(w, "  hash:     %s\n", hash)
	if len(migs) > 0 {
		fmt.Fprintf(w, "  migrations: %d (schema %s)\n", len(migs), schema)
	}
	if signature == "" {
		fmt.Fprintln(w, "  warning:  no signature (set exports.signature or --signature once build computes it)")
	}
	return nil
}

// collectMigrations reads the module's migrations/ directory (ordered) and the
// target schema from the manifest. Returns empty if the module has no migrations.
func collectMigrations(file string, m *manifest.Manifest, lib string) (schema string, migs []registry.NamedBlob, err error) {
	if m.Migrations == nil || m.Migrations.Dir == "" {
		return "", nil, nil
	}
	dir := filepath.Join(filepath.Dir(file), m.Migrations.Dir)
	loaded, err := sqlchan.LoadMigrations(dir)
	if err != nil {
		// no migrations dir is not an error; a real read failure is
		if os.IsNotExist(err) {
			return "", nil, nil
		}
		return "", nil, err
	}
	for _, mg := range loaded {
		data, rerr := os.ReadFile(mg.Path)
		if rerr != nil {
			return "", nil, rerr
		}
		migs = append(migs, registry.NamedBlob{Name: filepath.Base(mg.Path), Data: data})
	}
	schema = m.Migrations.Schema
	if schema == "" {
		schema = lib
	}
	return schema, migs, nil
}

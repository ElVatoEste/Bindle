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
)

func newPublishCmd() *cobra.Command {
	var file, regDir, artifact, signature string
	var force bool

	cmd := &cobra.Command{
		Use:   "publish",
		Short: "Publish a module's artifact and metadata to the registry",
		Long: "Write a module's artifact under <name>/<version>/ in the registry and " +
			"upsert its versions.json (version, signature, hash, dependencies).\n\n" +
			"Until `bindle build` exists, pass the artifact with --artifact.",
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			return runPublish(c.OutOrStdout(), file, regDir, artifact, signature, force)
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", manifest.FileName, "path to the manifest")
	cmd.Flags().StringVar(&regDir, "registry", "", "registry directory (overrides manifest registries.default)")
	cmd.Flags().StringVar(&artifact, "artifact", "", "path to the artifact (SAVF) to publish")
	cmd.Flags().StringVar(&signature, "signature", "", "ILE signature (defaults to exports.signature)")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an already-published version")
	return cmd
}

func runPublish(w io.Writer, file, regDir, artifactPath, signature string, force bool) error {
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

	rel, hash, err := registry.Open(root).Publish(registry.PublishInput{
		Name:         m.Name,
		Version:      m.Version,
		Signature:    signature,
		Dependencies: m.Dependencies,
		ArtifactName: filepath.Base(artifactPath),
		Artifact:     data,
	}, force)
	if err != nil {
		return err
	}

	fmt.Fprintf(w, "published %s@%s to %s\n", m.Name, m.Version, root)
	fmt.Fprintf(w, "  artifact: %s\n", rel)
	fmt.Fprintf(w, "  hash:     %s\n", hash)
	if signature == "" {
		fmt.Fprintln(w, "  warning:  no signature (set exports.signature or --signature once build computes it)")
	}
	return nil
}

// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/ElVatoEste/Bindle/internal/manifest"
	"github.com/ElVatoEste/Bindle/internal/registry"
	"github.com/ElVatoEste/Bindle/internal/resolver"
	"github.com/ElVatoEste/Bindle/internal/ui"
)

func newListCmd() *cobra.Command {
	var file, regDir string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "Inspect the resolved dependency graph",
		Long:  "Resolve the manifest against a registry and print the flat list of selected packages.",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			return runList(c.OutOrStdout(), file, regDir, false)
		},
	}
	cmd.PersistentFlags().StringVarP(&file, "file", "f", manifest.FileName, "path to the manifest")
	cmd.PersistentFlags().StringVar(&regDir, "registry", "", "registry directory (overrides manifest registries.default)")

	cmd.AddCommand(&cobra.Command{
		Use:   "tree",
		Short: "Print the dependency tree",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			return runList(c.OutOrStdout(), file, regDir, true)
		},
	})
	return cmd
}

func runList(w io.Writer, file, regDir string, asTree bool) error {
	m, err := manifest.Load(file)
	if err != nil {
		return err
	}
	root, err := registryDir(regDir, m)
	if err != nil {
		return err
	}
	res, err := resolver.Resolve(m, registry.Open(root))
	if err != nil {
		return err
	}
	if asTree {
		printTree(w, m, res)
	} else {
		printList(w, m, res)
	}
	return nil
}

// registryDir picks the registry location: the --registry flag wins, otherwise
// the manifest's default registry (with its URI scheme stripped to a local path).
func registryDir(flag string, m *manifest.Manifest) (string, error) {
	if flag != "" {
		return flag, nil
	}
	if uri, ok := m.Registries["default"]; ok {
		return localPath(uri), nil
	}
	return "", fmt.Errorf("no registry: pass --registry <dir> or set registries.default in %s", manifest.FileName)
}

func localPath(uri string) string {
	for _, scheme := range []string{"file://", "ifs://"} {
		if rest, ok := strings.CutPrefix(uri, scheme); ok {
			return filepath.FromSlash(rest)
		}
	}
	return uri
}

func printList(w io.Writer, m *manifest.Manifest, res *resolver.Resolution) {
	uo := ui.New(w)
	uo.Heading("%s %s", m.Name, m.Version)
	uo.Info("%s", uo.Dim(fmt.Sprintf("resolved %d package(s):", len(res.Selected))))

	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	for _, name := range res.Order { // install order: dependencies first
		fmt.Fprintf(tw, "  %s\t%s\n", uo.Bold(name), uo.Cyan(res.Selected[name].Version))
	}
	_ = tw.Flush()
}

func printTree(w io.Writer, m *manifest.Manifest, res *resolver.Resolution) {
	uo := ui.New(w)
	uo.Heading("%s %s", m.Name, m.Version)
	deps := sortedStrKeys(m.Dependencies)
	for i, name := range deps {
		printNode(uo, name, res, "", i == len(deps)-1)
	}
}

func printNode(uo *ui.Printer, name string, res *resolver.Resolution, prefix string, last bool) {
	branch, childPrefix := "├── ", prefix+"│   "
	if last {
		branch, childPrefix = "└── ", prefix+"    "
	}

	av, ok := res.Selected[name]
	version := "?"
	if ok {
		version = av.Version
	}
	uo.Info("%s%s %s", uo.Gray(prefix+branch), uo.Bold(name), uo.Cyan(version))

	children := sortedStrKeys(av.Dependencies)
	for i, child := range children {
		printNode(uo, child, res, childPrefix, i == len(children)-1)
	}
}

func sortedStrKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

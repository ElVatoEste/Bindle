// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/ElVatoEste/Bindle/internal/builder"
	"github.com/ElVatoEste/Bindle/internal/config"
	"github.com/ElVatoEste/Bindle/internal/manifest"
	"github.com/ElVatoEste/Bindle/internal/transport"
	"github.com/ElVatoEste/Bindle/internal/ui"
)

func newBuildCmd() *cobra.Command {
	var ov config.Overrides
	var configPath, file, lib, out string
	var keep bool

	cmd := &cobra.Command{
		Use:   "build",
		Short: "Compile a module on the IBM i host and package a SAVF",
		Long: "Upload the module's RPG sources to the IBM i host, compile them " +
			"(CRTRPGMOD + CRTSRVPGM), read the service program signature, package a " +
			"SAVF, and download it locally.",
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			return runBuild(c.OutOrStdout(), configPath, ov, file, lib, out, keep)
		},
	}
	profileFlags(cmd, &ov, &configPath)
	cmd.Flags().StringVarP(&file, "file", "f", manifest.FileName, "path to the module manifest")
	cmd.Flags().StringVar(&lib, "library", "", "target IBM i library (overrides manifest.library)")
	cmd.Flags().StringVar(&out, "out", "", "local path for the .savf (default .bindle/build/<srvpgm>.savf)")
	cmd.Flags().BoolVar(&keep, "keep", false, "keep remote work objects for debugging")
	return cmd
}

func runBuild(w io.Writer, configPath string, ov config.Overrides, file, lib, out string, keep bool) error {
	m, err := manifest.Load(file)
	if err != nil {
		return err
	}
	if m.Exports == nil || m.Exports.Srvpgm == "" {
		return fmt.Errorf("%s is not a buildable module (needs exports.srvpgm)", m.Name)
	}

	srcDir := filepath.Dir(file)
	if m.Build != nil && m.Build.Src != "" {
		srcDir = filepath.Join(filepath.Dir(file), m.Build.Src)
	}
	if out == "" {
		out = filepath.Join(".bindle", "build", m.Exports.Srvpgm+".savf")
	}

	uo := ui.New(w)
	p, err := resolveProfile(configPath, ov)
	if err != nil {
		return err
	}
	conn, err := transport.DialSSH(*p)
	if err != nil {
		return err
	}
	defer conn.Close()

	sp := uo.Spinner("building " + m.Name + "@" + m.Version)
	sp.Start()
	res, err := builder.Build(conn, builder.Options{
		Manifest:   m,
		SourceDir:  srcDir,
		TargetLib:  lib,
		OutputPath: out,
		Keep:       keep,
		Logf:       func(format string, a ...any) { sp.Update(format, a...) },
	})
	sp.Stop()
	if err != nil {
		uo.Fail("build failed")
		return err
	}

	uo.OK("built %s", uo.Bold(m.Name+"@"+m.Version))
	uo.KeyVal("library", res.TargetLib)
	uo.KeyVal("srvpgm", res.Srvpgm)
	uo.KeyVal("signature", uo.Cyan(res.Signature))
	uo.KeyVal("savf", res.SavfPath)
	uo.Info("")
	uo.Info("%s bindle publish -f %s --artifact %s --signature %s",
		uo.Dim("publish with:"), file, res.SavfPath, res.Signature)
	return nil
}

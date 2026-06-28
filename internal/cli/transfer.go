// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/ElVatoEste/Bindle/internal/config"
	"github.com/ElVatoEste/Bindle/internal/transport"
)

func newPutCmd() *cobra.Command {
	var ov config.Overrides
	var configPath string

	cmd := &cobra.Command{
		Use:   "put <local> <remote>",
		Short: "Upload a file to the IBM i host over SFTP",
		Args:  cobra.ExactArgs(2),
		RunE: func(c *cobra.Command, args []string) error {
			return runTransfer(c.OutOrStdout(), configPath, ov, true, args[0], args[1])
		},
	}
	profileFlags(cmd, &ov, &configPath)
	return cmd
}

func newGetCmd() *cobra.Command {
	var ov config.Overrides
	var configPath string

	cmd := &cobra.Command{
		Use:   "get <remote> <local>",
		Short: "Download a file from the IBM i host over SFTP",
		Args:  cobra.ExactArgs(2),
		RunE: func(c *cobra.Command, args []string) error {
			return runTransfer(c.OutOrStdout(), configPath, ov, false, args[0], args[1])
		},
	}
	profileFlags(cmd, &ov, &configPath)
	return cmd
}

func runTransfer(w io.Writer, configPath string, ov config.Overrides, upload bool, a, b string) error {
	p, err := resolveProfile(configPath, ov)
	if err != nil {
		return err
	}
	conn, err := transport.DialSSH(*p)
	if err != nil {
		return err
	}
	defer conn.Close()

	if upload {
		if err := conn.Upload(a, b); err != nil {
			return err
		}
		fmt.Fprintf(w, "uploaded %s -> %s\n", a, b)
		return nil
	}
	if err := conn.Download(a, b); err != nil {
		return err
	}
	fmt.Fprintf(w, "downloaded %s -> %s\n", a, b)
	return nil
}

// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"fmt"
	"io"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/ElVatoEste/Bindle/internal/config"
)

func newProfileCmd() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Inspect connection profiles (~/.bindle/config.json)",
	}
	cmd.PersistentFlags().StringVar(&configPath, "config", "", "path to config (default ~/.bindle/config.json)")

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List configured profiles",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			return runProfileList(c.OutOrStdout(), configPath)
		},
	})

	var ov config.Overrides
	show := &cobra.Command{
		Use:   "show",
		Short: "Show the resolved connection profile (defaults < file < env < flags)",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			return runProfileShow(c.OutOrStdout(), configPath, ov)
		},
	}
	show.Flags().StringVar(&ov.Profile, "profile", "", "profile name")
	show.Flags().StringVar(&ov.Host, "host", "", "override host")
	show.Flags().StringVar(&ov.User, "user", "", "override user")
	show.Flags().IntVar(&ov.Port, "port", 0, "override port")
	cmd.AddCommand(show)

	return cmd
}

func loadConfig(path string) (*config.Config, string, error) {
	if path == "" {
		p, err := config.DefaultPath()
		if err != nil {
			return nil, "", err
		}
		path = p
	}
	c, err := config.Load(path)
	return c, path, err
}

func runProfileList(w io.Writer, configPath string) error {
	c, path, err := loadConfig(configPath)
	if err != nil {
		return err
	}
	if len(c.Profiles) == 0 {
		fmt.Fprintf(w, "no profiles in %s\n", path)
		return nil
	}

	names := make([]string, 0, len(c.Profiles))
	for n := range c.Profiles {
		names = append(names, n)
	}
	sort.Strings(names)

	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "PROFILE\tHOST\tPORT\tUSER\tTRANSPORT")
	for _, n := range names {
		p := c.Profiles[n]
		marker := ""
		if n == c.DefaultProfile {
			marker = " (default)"
		}
		port := p.Port
		if port == 0 {
			port = config.DefaultPort
		}
		fmt.Fprintf(tw, "%s%s\t%s\t%d\t%s\t%s\n", n, marker, p.Host, port, p.User, transportOr(p.Transport))
	}
	return tw.Flush()
}

func runProfileShow(w io.Writer, configPath string, ov config.Overrides) error {
	c, _, err := loadConfig(configPath)
	if err != nil {
		return err
	}
	p, err := c.Resolve(ov)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "host:       %s\n", p.Host)
	fmt.Fprintf(w, "port:       %d\n", p.Port)
	fmt.Fprintf(w, "user:       %s\n", p.User)
	fmt.Fprintf(w, "transport:  %s\n", p.Transport)
	fmt.Fprintf(w, "auth:       %s\n", p.Auth)
	if p.KeyFile != "" {
		fmt.Fprintf(w, "keyFile:    %s\n", p.KeyFile)
	}
	if p.Password != "" {
		fmt.Fprintf(w, "password:   %s\n", "********")
	}
	if p.DefaultLibrary != "" {
		fmt.Fprintf(w, "library:    %s\n", p.DefaultLibrary)
	}
	return nil
}

func transportOr(t string) string {
	if t == "" {
		return "ssh"
	}
	return t
}

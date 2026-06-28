// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ElVatoEste/Bindle/internal/config"
	"github.com/ElVatoEste/Bindle/internal/transport"
	"github.com/ElVatoEste/Bindle/internal/ui"
)

func resolveProfile(configPath string, ov config.Overrides) (*config.Profile, error) {
	c, _, err := loadConfig(configPath)
	if err != nil {
		return nil, err
	}
	return c.Resolve(ov)
}

func profileFlags(cmd *cobra.Command, ov *config.Overrides, configPath *string) {
	cmd.Flags().StringVar(configPath, "config", "", "path to config (default ~/.bindle/config.json)")
	cmd.Flags().StringVar(&ov.Profile, "profile", "", "connection profile")
	cmd.Flags().StringVar(&ov.Host, "host", "", "override host")
	cmd.Flags().StringVar(&ov.User, "user", "", "override user")
	cmd.Flags().IntVar(&ov.Port, "port", 0, "override port")
}

func newPingCmd() *cobra.Command {
	var ov config.Overrides
	var configPath string

	cmd := &cobra.Command{
		Use:   "ping",
		Short: "Connect to the IBM i host and report basic info",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			return runPing(c.OutOrStdout(), configPath, ov)
		},
	}
	profileFlags(cmd, &ov, &configPath)
	return cmd
}

func runPing(w io.Writer, configPath string, ov config.Overrides) error {
	out := ui.New(w)
	p, err := resolveProfile(configPath, ov)
	if err != nil {
		return err
	}

	sp := out.Spinner(fmt.Sprintf("connecting to %s@%s:%d ...", p.User, p.Host, p.Port))
	sp.Start()
	conn, err := transport.DialSSH(*p)
	sp.Stop()
	if err != nil {
		out.Fail("connect %s@%s:%d: %v", p.User, p.Host, p.Port, err)
		return err
	}
	defer conn.Close()
	out.OK("connected to %s@%s:%d", out.Bold(p.User), p.Host, p.Port)

	if res, err := conn.Run("id"); err == nil && strings.TrimSpace(res.Stdout) != "" {
		out.KeyVal("identity", strings.TrimSpace(res.Stdout))
	}
	if res, err := conn.RunCL("DSPLIBL"); err == nil {
		for _, line := range strings.Split(res.Stdout, "\n") {
			if strings.Contains(line, "CUR ") {
				out.KeyVal("current lib", strings.Fields(line)[0])
			}
		}
	}

	// capability probes (non-fatal)
	report := func(label, cmd string) {
		r, e := conn.Run(cmd)
		ok := e == nil && !r.Failed() && strings.TrimSpace(r.Stdout) != ""
		if ok {
			out.OK("%s %s", padRightLabel(label), out.Gray("("+strings.TrimSpace(r.Stdout)+")"))
		} else {
			out.Warn("%s %s", padRightLabel(label), out.Gray("not found"))
		}
	}
	report("bob", "command -v makei 2>/dev/null || command -v bob 2>/dev/null")
	report("yum", "/QOpenSys/pkgs/bin/yum --version 2>/dev/null | head -1")
	report("git", "command -v git 2>/dev/null")
	return nil
}

func padRightLabel(s string) string {
	for len(s) < 5 {
		s += " "
	}
	return s
}

func newExecCmd() *cobra.Command {
	var ov config.Overrides
	var configPath string
	var asCL bool

	cmd := &cobra.Command{
		Use:   "exec [flags] -- <command>",
		Short: "Run a command (or CL with --cl) on the IBM i host",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			return runExec(c.OutOrStdout(), configPath, ov, asCL, strings.Join(args, " "))
		},
	}
	profileFlags(cmd, &ov, &configPath)
	cmd.Flags().BoolVar(&asCL, "cl", false, "run the argument as a CL command (via system)")
	return cmd
}

func runExec(w io.Writer, configPath string, ov config.Overrides, asCL bool, command string) error {
	p, err := resolveProfile(configPath, ov)
	if err != nil {
		return err
	}
	conn, err := transport.DialSSH(*p)
	if err != nil {
		return err
	}
	defer conn.Close()

	var res transport.Result
	if asCL {
		res, err = conn.RunCL(command)
	} else {
		res, err = conn.Run(command)
	}
	if err != nil {
		return err
	}

	if res.Stdout != "" {
		fmt.Fprint(w, res.Stdout)
		if !strings.HasSuffix(res.Stdout, "\n") {
			fmt.Fprintln(w)
		}
	}
	if res.Stderr != "" {
		fmt.Fprintf(w, "[stderr] %s", res.Stderr)
		if !strings.HasSuffix(res.Stderr, "\n") {
			fmt.Fprintln(w)
		}
	}
	if res.Failed() {
		return fmt.Errorf("remote command exited %d", res.ExitCode)
	}
	return nil
}

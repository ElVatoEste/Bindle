// SPDX-License-Identifier: GPL-3.0-or-later

package transport

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"github.com/ElVatoEste/Bindle/internal/config"
)

// SSH is a live connection to an IBM i host over SSH.
type SSH struct {
	client  *ssh.Client
	profile config.Profile
}

// Result is the outcome of a remote command.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Failed reports whether the command exited non-zero.
func (r Result) Failed() bool { return r.ExitCode != 0 }

// DialSSH opens an SSH connection described by the profile.
func DialSSH(p config.Profile) (*SSH, error) {
	if p.Transport != "" && p.Transport != "ssh" {
		return nil, fmt.Errorf("transport %q not supported yet (only ssh)", p.Transport)
	}
	auth, err := authMethods(p)
	if err != nil {
		return nil, err
	}

	port := p.Port
	if port == 0 {
		port = config.DefaultPort
	}
	cfg := &ssh.ClientConfig{
		User:    p.User,
		Auth:    auth,
		Timeout: 15 * time.Second,
		// TODO(v1): verify against ~/.ssh/known_hosts instead of trusting blindly.
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	addr := net.JoinHostPort(p.Host, strconv.Itoa(port))
	client, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect %s@%s: %w", p.User, addr, err)
	}
	return &SSH{client: client, profile: p}, nil
}

func authMethods(p config.Profile) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod

	if p.KeyFile != "" {
		key, err := os.ReadFile(expandHome(p.KeyFile))
		if err != nil {
			return nil, fmt.Errorf("read key %q: %w", p.KeyFile, err)
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, fmt.Errorf("parse key %q: %w", p.KeyFile, err)
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}
	if p.Password != "" {
		methods = append(methods, ssh.Password(p.Password))
	}
	if len(methods) == 0 {
		return nil, fmt.Errorf("no credentials: set a key (auth: key) or BINDLE_PASSWORD")
	}
	return methods, nil
}

// Run executes a command in the host's default (PASE/QSH) shell.
func (s *SSH) Run(cmd string) (Result, error) {
	sess, err := s.client.NewSession()
	if err != nil {
		return Result{}, fmt.Errorf("open session: %w", err)
	}
	defer sess.Close()

	var out, errb bytes.Buffer
	sess.Stdout = &out
	sess.Stderr = &errb

	runErr := sess.Run(cmd)
	res := Result{Stdout: out.String(), Stderr: errb.String()}
	var exitErr *ssh.ExitError
	if errors.As(runErr, &exitErr) {
		res.ExitCode = exitErr.ExitStatus()
		return res, nil
	}
	if runErr != nil {
		return res, fmt.Errorf("run %q: %w", cmd, runErr)
	}
	return res, nil
}

// RunCL runs a single CL command via the IBM i `system` utility.
func (s *SSH) RunCL(cl string) (Result, error) {
	quoted := strings.ReplaceAll(cl, `"`, `\"`)
	return s.Run(`system "` + quoted + `"`)
}

// Upload copies a local file to the host over SFTP.
func (s *SSH) Upload(localPath, remotePath string) error {
	c, err := sftp.NewClient(s.client)
	if err != nil {
		return fmt.Errorf("sftp: %w", err)
	}
	defer c.Close()

	src, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open %q: %w", localPath, err)
	}
	defer src.Close()

	if dir := filepathDir(remotePath); dir != "" {
		_ = c.MkdirAll(dir)
	}
	dst, err := c.Create(remotePath)
	if err != nil {
		return fmt.Errorf("create remote %q: %w", remotePath, err)
	}
	defer dst.Close()

	if _, err := dst.ReadFrom(src); err != nil {
		return fmt.Errorf("upload %q: %w", remotePath, err)
	}
	return nil
}

// Download copies a remote file to the local filesystem over SFTP.
func (s *SSH) Download(remotePath, localPath string) error {
	c, err := sftp.NewClient(s.client)
	if err != nil {
		return fmt.Errorf("sftp: %w", err)
	}
	defer c.Close()

	src, err := c.Open(remotePath)
	if err != nil {
		return fmt.Errorf("open remote %q: %w", remotePath, err)
	}
	defer src.Close()

	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return err
	}
	dst, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("create %q: %w", localPath, err)
	}
	defer dst.Close()

	if _, err := src.WriteTo(dst); err != nil {
		return fmt.Errorf("download %q: %w", remotePath, err)
	}
	return nil
}

// Close ends the SSH connection.
func (s *SSH) Close() error { return s.client.Close() }

func expandHome(path string) string {
	if strings.HasPrefix(path, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~"))
		}
	}
	return path
}

// filepathDir returns the POSIX directory of a remote path (always forward slashes).
func filepathDir(p string) string {
	i := strings.LastIndex(p, "/")
	if i <= 0 {
		return ""
	}
	return p[:i]
}

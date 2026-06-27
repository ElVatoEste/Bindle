// SPDX-License-Identifier: GPL-3.0-or-later

package transport

import (
	"testing"

	"github.com/ElVatoEste/Bindle/internal/config"
)

func TestResultFailed(t *testing.T) {
	if (Result{ExitCode: 0}).Failed() {
		t.Error("exit 0 should not be failed")
	}
	if !(Result{ExitCode: 2}).Failed() {
		t.Error("exit 2 should be failed")
	}
}

func TestAuthMethodsRequiresCredentials(t *testing.T) {
	if _, err := authMethods(config.Profile{User: "u", Host: "h"}); err == nil {
		t.Error("expected error when no key or password is set")
	}
	if _, err := authMethods(config.Profile{Password: "p"}); err != nil {
		t.Errorf("password should be accepted: %v", err)
	}
}

func TestFilepathDir(t *testing.T) {
	cases := map[string]string{
		"/home/v/x.savf": "/home/v",
		"x.savf":         "",
		"/x.savf":        "",
		"a/b/c":          "a/b",
	}
	for in, want := range cases {
		if got := filepathDir(in); got != want {
			t.Errorf("filepathDir(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDialRejectsUnknownTransport(t *testing.T) {
	if _, err := DialSSH(config.Profile{Host: "h", User: "u", Password: "p", Transport: "odbc"}); err == nil {
		t.Error("expected error for unsupported transport")
	}
}

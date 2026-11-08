// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package shellwrap

import (
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
)

func TestSkipReasonByShape(t *testing.T) {
	denyAll := func(string) bool { return true }
	cases := []struct {
		name    string
		enabled bool
		env     map[string]string
		command string
		blocked func(string) bool
		want    string
	}{
		{name: "wrapped command has no skip", enabled: true, command: "go test ./...", blocked: never, want: ""},
		{name: "unknown binary", enabled: true, command: "vim README.md", blocked: never, want: execwrap.SkipNotAllowlisted},
		{name: "and-list", enabled: true, command: "vim a && vim b", blocked: never, want: execwrap.SkipMetachar},
		{name: "or-list", enabled: true, command: "vim a || echo b", blocked: never, want: execwrap.SkipMetachar},
		{name: "or-true on an unknown binary is not a metachar", enabled: true, command: "vim a || true", blocked: never, want: execwrap.SkipNotAllowlisted},
		{name: "semicolon", enabled: true, command: "vim a; vim b", blocked: never, want: execwrap.SkipMetachar},
		{name: "pipe", enabled: true, command: "git log | head", blocked: never, want: execwrap.SkipMetachar},
		{name: "closed quote is wrapped", enabled: true, command: "git commit -m 'x'", blocked: never, want: ""},
		{name: "unclosed quote", enabled: true, command: "git commit -m 'x", blocked: never, want: execwrap.SkipMetachar},
		{name: "dollar", enabled: true, command: "echo $HOME", blocked: never, want: execwrap.SkipMetachar},
		{name: "deny rule", enabled: true, command: "go test ./...", blocked: denyAll, want: execwrap.SkipDenied},
		{name: "kill switch", enabled: true, env: map[string]string{execwrap.KillSwitchEnv: "0"}, command: "go test ./...", blocked: never, want: execwrap.SkipDisabled},
		{name: "config off", enabled: false, command: "go test ./...", blocked: never, want: execwrap.SkipDisabled},
		{name: "absolute path to allowlisted binary", enabled: true, command: "/opt/homebrew/bin/gh pr list", blocked: never, want: execwrap.SkipAbsPath},
		{name: "absolute path to unknown binary", enabled: true, command: "/usr/bin/vim x", blocked: never, want: execwrap.SkipNotAllowlisted},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			withEnv(t, c.enabled, c.env, true)
			d := decide(core.ShellPOSIX, c.command, c.blocked, true)
			if got := execwrap.SkipReason(d, c.command); got != c.want {
				t.Fatalf("skip = %q, want %q (decision %+v)", got, c.want, d)
			}
		})
	}
}

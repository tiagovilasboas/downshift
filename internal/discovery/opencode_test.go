// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestParseOpenCodeList(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantN   int
		wantErr bool
	}{
		{name: "provider model rows", in: "github-copilot/claude-haiku-5.5\nopencode/big-pickle\n", wantN: 2},
		{name: "blank lines ignored", in: "\nopencode/big-pickle\n\n", wantN: 1},
		{name: "ansi stripped", in: "\x1b[32mopencode/big-pickle\x1b[0m\n", wantN: 1},
		{name: "empty output", in: "", wantErr: true},
		{name: "row without provider", in: "big-pickle\n", wantErr: true},
		{name: "prose line", in: "opencode/big-pickle\nTip: run opencode run\n", wantErr: true},
		{name: "duplicate row", in: "opencode/a\nopencode/a\n", wantErr: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parseOpenCodeList([]byte(c.in))
			if (err != nil) != c.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, c.wantErr)
			}
			if err == nil && len(got) != c.wantN {
				t.Errorf("models = %d, want %d", len(got), c.wantN)
			}
		})
	}
}

// fakeOpenCode writes a shell stand-in for the opencode binary. The test
// replaces the trust check, because the fake lives in a temp directory.
func fakeOpenCode(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "opencode")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body), 0o700); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestDefaultsIncludeOpenCodeSource(t *testing.T) {
	for _, s := range Defaults() {
		if s.Harness() == "opencode" {
			return
		}
	}
	t.Fatal("Defaults() has no OpenCode source: opencode would never be discovered")
}

func TestOpenCodeCLIUsesFakeBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake executables are shell scripts")
	}
	bin := fakeOpenCode(t, "echo opencode/big-pickle\necho github-copilot/claude-haiku-5.5\n")
	prev := trustedBinaryFn
	trustedBinaryFn = func(string) bool { return true }
	t.Cleanup(func() { trustedBinaryFn = prev })
	got, err := OpenCodeCLI{Bin: bin}.Discover(t.Context())
	if err != nil || len(got) != 2 {
		t.Fatalf("Discover = %v, %v", got, err)
	}
}

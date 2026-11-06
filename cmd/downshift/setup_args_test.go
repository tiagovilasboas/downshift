// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestSetupUsageMentionsMcp(t *testing.T) {
	var out, errw bytes.Buffer
	if code := runSetup([]string{"-h"}, strings.NewReader(""), &out, &errw); code != 0 || !strings.Contains(out.String(), "--no-mcp") {
		t.Errorf("usage lacks --no-mcp: %d\n%s", code, out.String())
	}
}

func TestSetupArgsMcpIsOnUnlessNoMcp(t *testing.T) {
	in := strings.NewReader("")
	for _, tc := range []struct {
		args []string
		want bool
	}{
		{nil, true},
		{[]string{"--no-mcp"}, false},
		{[]string{"--mcp"}, true},
		{[]string{"--mcp", "--no-mcp"}, false},
		{[]string{"--no-mcp", "--mcp"}, true},
	} {
		opts := defaultSetupOptions(in, io.Discard, io.Discard)
		if _, _, bad := parseSetupArgs(tc.args, &opts); bad != "" {
			t.Fatalf("%v: unexpected bad flag %q", tc.args, bad)
		}
		if opts.Mcp != tc.want {
			t.Errorf("%v: Mcp = %v, want %v", tc.args, opts.Mcp, tc.want)
		}
	}
}

func TestSetupArgsReportsUnknownFlagAndHelp(t *testing.T) {
	opts := defaultSetupOptions(strings.NewReader(""), io.Discard, io.Discard)
	if _, _, bad := parseSetupArgs([]string{"--bogus"}, &opts); bad != "--bogus" {
		t.Errorf("bad = %q, want --bogus", bad)
	}
	if _, help, _ := parseSetupArgs([]string{"--help"}, &opts); !help {
		t.Error("--help not reported as help")
	}
}

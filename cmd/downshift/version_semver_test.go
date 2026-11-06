// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"runtime/debug"
	"strings"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b   string
		want   int
		wantOK bool
	}{
		{"v0.7.0", "v0.8.1", -1, true},
		{"0.8.1", "v0.8.1", 0, true},
		{"v0.10.0", "v0.9.0", 1, true},
		{"v0.8.1-rc1", "v0.8.1", -1, true},
		{"v0.8.2-0.20261105-abcdef", "v0.8.1", 1, true},
		{"v0.0.0-20261105-abcdef", "v0.8.1", -1, true},
		{"v0.8.1+meta", "v0.8.1", 0, true},
		{"v1.0.0-rc1", "v1.0.0-rc2", -1, true},
		{"v1.0.0-rc2", "v1.0.0-rc1", 1, true},
		{"v1.0.0-rc1", "v1.0.0-rc1", 0, true},
		{"v1.0.0-alpha", "v1.0.0-alpha.1", -1, true},
		{"v1.0.0-alpha.1", "v1.0.0-alpha.beta", -1, true},
		{"v1.0.0-alpha.beta", "v1.0.0-beta", -1, true},
		{"v1.0.0-beta.2", "v1.0.0-beta.11", -1, true},
		{"v1.0.0-beta.11", "v1.0.0-rc.1", -1, true},
		{"v1.0.0-rc.1", "v1.0.0", -1, true},
		{"v0.8.2-0.20261105-abcdef", "v0.8.2-0.20261106-abcdef", -1, true},
		{"dev", "v0.8.1", 0, false},
		{"(devel)", "v0.8.1", 0, false},
		{"", "v0.8.1", 0, false},
		{"v1.2", "v1.2.3", 0, false},
	}
	for _, c := range cases {
		got, ok := compareVersions(c.a, c.b)
		if got != c.want || ok != c.wantOK {
			t.Errorf("compareVersions(%q,%q) = %d,%v want %d,%v", c.a, c.b, got, ok, c.want, c.wantOK)
		}
	}
}

func TestResolveBuild(t *testing.T) {
	info := func(ver string, rev string) *debug.BuildInfo {
		return &debug.BuildInfo{Main: debug.Module{Version: ver},
			Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: rev}}}
	}
	cases := []struct {
		name, ver, commit string
		info              *debug.BuildInfo
		wantVer, wantComm string
	}{
		{"ldflags win", "v0.9.0", "abc", info("v0.8.1", "zzz"), "v0.9.0", "abc"},
		{"go install", "dev", "", info("v0.8.1", ""), "v0.8.1", ""},
		{"empty stamp", "", "", info("v0.8.1", "r1"), "v0.8.1", "r1"},
		{"devel module", "dev", "", info("(devel)", "r1"), "dev", "r1"},
		{"no build info", "dev", "", nil, "dev", ""},
		{"pseudo", "dev", "", info("v0.8.2-0.2026-abcdef", ""), "v0.8.2-0.2026-abcdef", ""},
	}
	for _, c := range cases {
		v, cm := resolveBuild(c.ver, c.commit, c.info)
		if v != c.wantVer || cm != c.wantComm {
			t.Errorf("%s: got %q,%q want %q,%q", c.name, v, cm, c.wantVer, c.wantComm)
		}
	}
}

func TestRunVersionPrintsLine(t *testing.T) {
	var b bytes.Buffer
	if runVersion(&b) != 0 || !strings.HasPrefix(b.String(), "downshift ") {
		t.Fatalf("got %q", b.String())
	}
}

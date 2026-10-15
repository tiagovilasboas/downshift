// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestUsageShortIsShortAndPointsToAll(t *testing.T) {
	var short, full bytes.Buffer
	usageShort(&short)
	usage(&full)
	if short.Len() >= full.Len()/2 {
		t.Errorf("short help (%d bytes) should be well under the full one (%d)", short.Len(), full.Len())
	}
	if !strings.Contains(short.String(), "help --all") {
		t.Error("short help must point to help --all")
	}
	for _, cmd := range []string{"doctor", "try", "stats"} {
		if !strings.Contains(short.String(), "downshift "+cmd) {
			t.Errorf("short help is missing %q", cmd)
		}
	}
	for _, maint := range []string{"eval-adapt", "train", "shadow-report", "context "} {
		if strings.Contains(short.String(), maint) {
			t.Errorf("short help must not list maintainer command %q", maint)
		}
	}
}

// The default help is a contract for a new user: eight commands, one per step
// of the install-check-use-fix loop. Everything else is in help --all.
func TestUsageShortListsTheEightEverydayCommands(t *testing.T) {
	var short, full bytes.Buffer
	usageShort(&short)
	usage(&full)
	out := short.String()
	for _, cmd := range []string{"setup", "doctor", "try", "stats", "report", "session", "config", "feedback"} {
		if !strings.Contains(out, "  downshift "+cmd) {
			t.Errorf("short help is missing the everyday command %q", cmd)
		}
	}
	for _, rest := range []string{"downshift models ", "downshift quota ", "downshift watch", "stats --detail", "stats --export"} {
		if strings.Contains(out, rest) {
			t.Errorf("short help should leave %q to help --all", rest)
		}
		if !strings.Contains(full.String(), strings.TrimSpace(rest)) {
			t.Errorf("help --all lost %q", rest)
		}
	}
	var cmds []string
	commands, _, _ := strings.Cut(out, "Examples")
	for _, line := range strings.Split(commands, "\n") {
		for _, c := range []string{"setup", "doctor", "try", "stats", "report", "session", "config", "feedback"} {
			if strings.HasPrefix(line, "  downshift "+c+" ") {
				cmds = append(cmds, c)
			}
		}
	}
	if len(cmds) != 8 {
		t.Errorf("short help lists the everyday commands %v, want exactly 8 lines", cmds)
	}
}

func TestWantsAllHelp(t *testing.T) {
	for _, c := range []struct {
		args []string
		want bool
	}{{nil, false}, {[]string{"--all"}, true}, {[]string{"-a"}, true}, {[]string{"foo"}, false}} {
		if got := wantsAllHelp(c.args); got != c.want {
			t.Errorf("%v: got %v", c.args, got)
		}
	}
}

// Copyright (c) 2026 Tiago Vilas Boas.
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

// help --all is grouped by who needs the command. Reorganising it must not lose
// a command, and each group has to hold the right ones.
func TestUsageAllIsGroupedAndComplete(t *testing.T) {
	var buf bytes.Buffer
	usage(&buf)
	out := buf.String()

	order := []string{"Everyday\n", "More about what happened\n", "Models and quota\n", "Plumbing", "Maintainer", "Experimental"}
	last := -1
	for _, h := range order {
		i := strings.Index(out, "\n"+h)
		if i < 0 {
			t.Fatalf("help --all has no %q section", strings.TrimSpace(h))
		}
		if i < last {
			t.Errorf("section %q is out of order", strings.TrimSpace(h))
		}
		last = i
	}

	section := func(name string) string {
		i := strings.Index(out, "\n"+name)
		rest := out[i+1:]
		if j := strings.Index(rest, "\n\n"); j >= 0 {
			rest = rest[:j]
		}
		return rest
	}
	for name, cmds := range map[string][]string{
		"Everyday":     {"downshift setup", "downshift doctor", "downshift try", "downshift stats ", "downshift report", "downshift session", "downshift config", "downshift feedback list"},
		"Plumbing":     {"downshift claude-code", "downshift cursor", "downshift codex", "downshift antigravity", "downshift kirocrew", "downshift quota import", "downshift quota collect", "downshift quota claude-statusline"},
		"Maintainer":   {"downshift benchmark", "downshift eval-outcome", "downshift eval-adapt", "downshift train", "downshift shadow-report", "downshift models check", "downshift models pull"},
		"Experimental": {"downshift context status", "downshift context compress", "dsmon"},
	} {
		sec := section(name)
		for _, c := range cmds {
			if !strings.Contains(sec, c) {
				t.Errorf("section %s should list %q", name, c)
			}
		}
	}

	// Every command the earlier flat list had is still here.
	for _, c := range []string{
		"downshift setup", "downshift doctor", "downshift try", "downshift stats", "downshift stats --detail", "downshift stats --export",
		"downshift report", "downshift session", "downshift watch", "downshift config",
		"downshift models list", "downshift models discover", "downshift models scores", "downshift models check", "downshift models pull",
		"downshift quota status", "downshift feedback list", "downshift feedback stats", "downshift feedback <id>",
		"downshift context benchmark", "downshift context metrics", "downshift context providers", "downshift context enable", "downshift context disable",
		"downshift version", "Grok note",
	} {
		if !strings.Contains(out, c) {
			t.Errorf("help --all lost %q", c)
		}
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

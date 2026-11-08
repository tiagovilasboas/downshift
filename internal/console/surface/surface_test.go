// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package surface

import (
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/console/shared/command"
	"github.com/tiagovilasboas/downshift-ai/internal/core"
)

var testSpecs = []command.Spec{
	{Args: []string{"report"}},
	{Args: []string{"setup"}, Writes: true},
	{Args: []string{"session"}, Terminal: true},
}

func envOf(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestDecide(t *testing.T) {
	tty := func(args []string, env map[string]string) Facts {
		return Facts{Args: args, StdinTTY: true, StdoutTTY: true, Getenv: envOf(env)}
	}
	cases := []struct {
		name string
		f    Facts
		want Decision
	}{
		{"bare on terminal", tty(nil, nil), Decision{true, "console"}},
		{"read-only spec", tty([]string{"report"}, nil), Decision{true, "console"}},
		{"plain env", tty(nil, map[string]string{"DOWNSHIFT_PLAIN": "1"}), Decision{false, "env:DOWNSHIFT_PLAIN"}},
		{"json flag", tty([]string{"report", "--json"}, nil), Decision{false, "flag:--json"}},
		{"flag with value", tty([]string{"report", "--export=x"}, nil), Decision{false, "flag:--export"}},
		{"short o", tty([]string{"-o"}, nil), Decision{false, "flag:-o"}},
		{"writing spec", tty([]string{"setup"}, nil), Decision{false, "command"}},
		{"terminal spec", tty([]string{"session"}, nil), Decision{false, "command"}},
		{"unknown command", tty([]string{"nope"}, nil), Decision{false, "command"}},
		{"spec plus extra arg", tty([]string{"report", "x"}, nil), Decision{false, "command"}},
		{"stdin not tty", Facts{StdoutTTY: true}, Decision{false, "no-tty"}},
		{"stdout not tty", Facts{StdinTTY: true}, Decision{false, "no-tty"}},
		{"ci", tty(nil, map[string]string{"CI": "true"}), Decision{false, "ci"}},
		{"dumb term", tty(nil, map[string]string{"TERM": "dumb"}), Decision{false, "term-dumb"}},
		{"agent env", tty(nil, map[string]string{"OPENCODE": "1"}), Decision{false, "agent-env:OPENCODE"}},
		{"session env", tty(nil, map[string]string{"CLAUDE_CODE_SESSION_ID": "s"}), Decision{false, "agent-env:CLAUDE_CODE_SESSION_ID"}},
		{"nil getenv", Facts{StdinTTY: true, StdoutTTY: true}, Decision{true, "console"}},
	}
	for _, c := range cases {
		if got := Decide(c.f, testSpecs); got != c.want {
			t.Errorf("%s: got %+v, want %+v", c.name, got, c.want)
		}
	}
}

func TestDecideFirstRuleWins(t *testing.T) {
	f := Facts{Args: []string{"setup", "--json"}, Getenv: envOf(map[string]string{"DOWNSHIFT_PLAIN": "1", "CI": "1"})}
	if got := Decide(f, testSpecs); got.Reason != "env:DOWNSHIFT_PLAIN" {
		t.Fatalf("got %+v", got)
	}
}

// Every declared agent variable keeps the console closed.
func TestDecideEveryAgentVarIsPlain(t *testing.T) {
	for _, n := range core.AllHarnessCaps() {
		for _, v := range agentVars(n.Caps) {
			f := Facts{StdinTTY: true, StdoutTTY: true, Getenv: envOf(map[string]string{v: "x"})}
			if Decide(f, nil).Console {
				t.Errorf("%s: %s set but console opened", n.Harness, v)
			}
		}
	}
}

// A harness id or hook name given as the command never opens the console, even
// on a terminal, because it is not a read-only console entry.
func TestDecideHarnessNamesStayPlain(t *testing.T) {
	names := []string{"hook", "pretool", "end"}
	for _, n := range core.AllHarnessCaps() {
		names = append(names, n.Harness)
	}
	for _, n := range names {
		for _, args := range [][]string{{n}, {"hook", n}} {
			if Decide(Facts{Args: args, StdinTTY: true, StdoutTTY: true}, testSpecs).Console {
				t.Errorf("args %v opened the console", args)
			}
		}
	}
}

// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"
)

// TestEveryCommandAnswersHelp is the contract for --help: every command and
// subcommand in the table exits 0 and prints a purpose line, Usage, Flags,
// Examples and a Next line naming what to run next.
func TestEveryCommandAnswersHelp(t *testing.T) {
	for name := range helpTopics {
		t.Run(name, func(t *testing.T) {
			var out, errw bytes.Buffer
			code, handled := commandHelpRequest(append(strings.Fields(name), "--help"), &out, &errw)
			if !handled || code != 0 {
				t.Fatalf("%s --help: handled=%v code=%d, want handled and 0", name, handled, code)
			}
			text := out.String()
			for _, section := range []string{"Usage:", "Flags:", "Examples:", "\nNext: "} {
				if !strings.Contains(text, section) {
					t.Errorf("%s --help lacks %q", name, strings.TrimSpace(section))
				}
			}
			if first, _, _ := strings.Cut(text, "\n"); !strings.HasPrefix(first, "downshift ") {
				t.Errorf("%s --help first line %q should start with the command name", name, first)
			}
			if errw.Len() != 0 {
				t.Errorf("%s --help wrote to stderr: %q", name, errw.String())
			}
		})
	}
}

// TestHelpShortFormAndHelpCommand checks -h, and `help <command>`, which show
// the same text as --help.
func TestHelpShortFormAndHelpCommand(t *testing.T) {
	for _, args := range [][]string{{"stats", "-h"}, {"help", "stats"}} {
		var out bytes.Buffer
		code, handled := commandHelpRequest(args, &out, &bytes.Buffer{})
		if !handled || code != 0 || out.String() != helpTopics["stats"] {
			t.Errorf("%v: handled=%v code=%d, want the stats help text", args, handled, code)
		}
	}
}

// TestHelpFlagAfterDoubleDashBelongsToWrappedProgram: compact passes what
// follows "--" to the program it wraps, so its --help is not ours.
func TestHelpFlagAfterDoubleDashBelongsToWrappedProgram(t *testing.T) {
	var out bytes.Buffer
	if _, handled := commandHelpRequest([]string{"compact", "--", "go", "test", "--help"}, &out, &bytes.Buffer{}); handled {
		t.Errorf("compact -- go test --help was answered by downshift: %q", out.String())
	}
}

// TestUnknownCommandIsOneLineWithSuggestion checks the error path: one line,
// a "Did you mean" when a real command is near, and no full help dump.
func TestUnknownCommandIsOneLineWithSuggestion(t *testing.T) {
	cases := []struct {
		name, want string
	}{
		{"statz", `unknown command "statz". Did you mean "stats"? Run ` + "`downshift help`."},
		{"reprot", `unknown command "reprot". Did you mean "report"? Run ` + "`downshift help`."},
		{"nosuch", `unknown command "nosuch". Run ` + "`downshift help`."},
	}
	for _, c := range cases {
		got := unknownCommandLine(c.name)
		if got != c.want {
			t.Errorf("unknownCommandLine(%q) = %q, want %q", c.name, got, c.want)
		}
		if strings.Contains(got, "\n") {
			t.Errorf("unknownCommandLine(%q) spans several lines", c.name)
		}
	}
}

// TestUnknownCommandNeverSuggestsLegacyNames: a typo must not be corrected to
// a name kept only for old hook and shell files.
func TestUnknownCommandNeverSuggestsLegacyNames(t *testing.T) {
	for _, typo := range []string{"exce", "compactr", "shell-ini"} {
		if got := unknownCommandLine(typo); strings.Contains(got, `"exec"`) ||
			strings.Contains(got, `"compactor"`) || strings.Contains(got, `"shell-init"`) {
			t.Errorf("unknownCommandLine(%q) suggested a legacy name: %s", typo, got)
		}
	}
}

// TestHelpTopicsAreUnique: mergeTopics overwrites on a repeated key, so a
// duplicate would silently drop one command's help.
func TestHelpTopicsAreUnique(t *testing.T) {
	total := len(dailyHelp) + len(modelsHelp) + len(hookHelpTopics) + len(maintainerHelp) + len(experimentalHelp)
	if total != len(helpTopics) {
		t.Errorf("help groups hold %d entries but the merged table has %d: a key is in two groups", total, len(helpTopics))
	}
}

// TestHelpFlagsUseDoubleDash: flags are written with "--". The two single-dash
// names that exist in the code (-q, -y) are the only exceptions.
func TestHelpFlagsUseDoubleDash(t *testing.T) {
	allowed := map[string]bool{"-q": true, "-y": true}
	for name, text := range helpTopics {
		inFlags := false
		for _, line := range strings.Split(text, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "Flags:" {
				inFlags = true
				continue
			}
			if inFlags && trimmed == "" {
				inFlags = false
			}
			if !inFlags || !strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "--") {
				continue
			}
			if flag, _, _ := strings.Cut(trimmed, " "); !allowed[flag] {
				t.Errorf("%s: flag %q should use a double dash", name, flag)
			}
		}
	}
}

// TestEveryDispatchedCommandIsInHelp reads main's switch and the hook table
// from source, so a command added to dispatch without help text fails here.
func TestEveryDispatchedCommandIsInHelp(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	dispatched := map[string]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		sw, ok := n.(*ast.SwitchStmt)
		if !ok {
			return true
		}
		for _, stmt := range sw.Body.List {
			cc, ok := stmt.(*ast.CaseClause)
			if !ok {
				continue
			}
			for _, e := range cc.List {
				if lit, ok := e.(*ast.BasicLit); ok {
					if s, err := strconv.Unquote(lit.Value); err == nil {
						dispatched[s] = true
					}
				}
			}
		}
		return true
	})
	for name := range harnessRegistry {
		if harnessRegistry[name].Hook != nil {
			dispatched[name] = true
		}
	}
	for name := range dispatched {
		if name == "-h" || name == "--help" || name == "--version" {
			continue
		}
		if _, ok := helpTopics[name]; !ok {
			if _, legacy := legacyNames[name]; !legacy {
				t.Errorf("dispatched command %q has no help entry", name)
			}
		}
	}
}

// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"context"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"
)

// codexRecipes plans the read-only and write recipes with every optional field
// set, so each flag a recipe can emit is in the argv under test.
func codexRecipes(t *testing.T) map[string]argv {
	t.Helper()
	spec := Spec{Dir: t.TempDir(), Model: "m-1", Effort: "high"}
	out := map[string]argv{}
	for name, plan := range map[string]planner{"read": codexReadOnly, "write": codexWrite} {
		a, err := plan(spec)
		if err != nil {
			t.Fatal(err)
		}
		if a.cleanup != nil {
			t.Cleanup(a.cleanup)
		}
		out[name] = a
	}
	return out
}

// flagsOf returns the option tokens of an argv: every token that starts with a
// dash except the lone "-" that means "read the prompt from stdin".
func flagsOf(args []string) []string {
	var flags []string
	for _, a := range args {
		if strings.HasPrefix(a, "-") && a != "-" {
			flags = append(flags, a)
		}
	}
	return flags
}

// codexHelp runs "codex ARGS --help". Help only: no model is called.
func codexHelp(t *testing.T, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "codex", append(args, "--help")...).CombinedOutput()
	if err != nil {
		t.Fatalf("codex %s --help: %v", strings.Join(args, " "), err)
	}
	return string(out)
}

// helpListsFlag reports whether a help text declares the flag as an option,
// either alone or in a "-c, --config" style pair.
func helpListsFlag(help, flag string) bool {
	re := regexp.MustCompile(`(?m)(^|[\s,])` + regexp.QuoteMeta(flag) + `([\s,=<\[]|$)`)
	return re.MatchString(help)
}

// TestCodexRecipeFlagsExistInTheInstalledCLI checks every flag in the read,
// write and resume argv against the installed codex's own help, so a flag
// the CLI does not know (such as "exec -a") fails here, not in a worker run.
// Skipped when codex is not on PATH.
func TestCodexRecipeFlagsExistInTheInstalledCLI(t *testing.T) {
	if _, err := exec.LookPath("codex"); err != nil {
		t.Skip("codex is not on PATH")
	}
	execHelp := codexHelp(t, "exec")
	resumeHelp := codexHelp(t, "exec", "resume")
	for mode, a := range codexRecipes(t) {
		t.Run(mode+" launch", func(t *testing.T) {
			for _, f := range flagsOf(a.args) {
				if !helpListsFlag(execHelp, f) {
					t.Errorf("codex exec --help does not list %s (argv %v)", f, a.args)
				}
			}
		})
		t.Run(mode+" resume", func(t *testing.T) {
			for _, f := range flagsOf(a.resume("sid-1")) {
				if !helpListsFlag(resumeHelp, f) {
					t.Errorf("codex exec resume --help does not list %s", f)
				}
			}
		})
	}
}

// TestCodexRecipesNeverBypassOrAskFlags holds the safety line on every Codex
// argv: no --dangerously-* flag, and no -a / --ask-for-approval (which codex
// exec rejects; the approval policy is a -c override).
func TestCodexRecipesNeverBypassOrAskFlags(t *testing.T) {
	for mode, a := range codexRecipes(t) {
		for label, args := range map[string][]string{"launch": a.args, "resume": a.resume("sid-1")} {
			for _, f := range flagsOf(args) {
				if strings.HasPrefix(f, "--dangerously") || f == "-a" || f == "--ask-for-approval" ||
					strings.HasPrefix(f, "--ask-for-approval=") {
					t.Errorf("codex %s %s argv has the forbidden flag %s: %v", mode, label, f, args)
				}
			}
		}
	}
}

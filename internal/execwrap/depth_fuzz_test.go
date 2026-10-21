// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"os/exec"
	"strings"
	"testing"
)

// The child sees the recursion guard.
func TestRun_ChildGetsDepthGuard(t *testing.T) {
	fakeBin(t, "go", `echo "depth=$DOWNSHIFT_EXEC_DEPTH"`)
	_, out, _ := run(t, []string{"go", "test"}, true)
	if out != "depth=1\n" {
		t.Fatalf("out = %q", out)
	}
}

// FuzzDecide holds the policy invariants for arbitrary input: a rewrite only
// inserts the quoted binary and " compact -- " before words of the original,
// the only metacharacters it lets through are the separators and the pipe
// suffix of policy_chain.go and policy_compound.go, a command with no
// metacharacter is wrapped only when allowlisted, and a wrapped command is
// never wrapped again.
func FuzzDecide(f *testing.F) {
	for _, seed := range []string{"go test ./...", "git status", "go test $(x)", "git status; id", "go test 'a b'", "go\ttest", "go test ./a\nb", "", "gofmt -l x; go test ./... 2>&1 | tail -5", "a && b ; go build"} {
		f.Add(seed)
	}
	const fuzzExe = "/opt/ds/downshift"
	const prefix = "'" + fuzzExe + "'" + execTail
	f.Fuzz(func(t *testing.T, command string) {
		d := Decide(command, fuzzExe)
		if !d.Rewrite {
			if d.Command != "" {
				t.Fatalf("Command set without Rewrite: %+v", d)
			}
			return
		}
		trimmed := strings.Trim(command, " ")
		simple := true
		for i := 0; i < len(trimmed); i++ {
			if !safeByte(trimmed[i]) {
				simple = false
				if !strings.ContainsRune(";&|>", rune(trimmed[i])) {
					t.Fatalf("unsafe byte %q accepted in %q", trimmed[i], command)
				}
			}
		}
		stripped := strings.ReplaceAll(d.Command, prefix, "")
		if strings.Join(strings.Fields(stripped), " ") != strings.Join(strings.Fields(trimmed), " ") {
			t.Fatalf("Command = %q changes the words of %q", d.Command, command)
		}
		if simple && FamilyOf(strings.Fields(trimmed)) == FamilyNone {
			t.Fatalf("rewrote a non-allowlisted command %q", command)
		}
		if again := Decide(d.Command, fuzzExe); again.Rewrite {
			t.Fatalf("wrapped command was wrapped again: %q", d.Command)
		}
	})
}

// The shell must see exactly the original words after the quoted binary.
func TestDecide_ShellSplitsWrappedCommandIntoOriginalWords(t *testing.T) {
	for _, c := range []string{"go test ./...", "go test -count=1 -run TestX ./internal/core", "git status -s -b", "go  test   ./a/b"} {
		d := Decide(c, "/opt/my dir/downshift")
		if !d.Rewrite {
			t.Fatalf("%q not rewritten", c)
		}
		out, err := exec.Command("sh", "-c", `for a in `+d.Command+`; do printf '[%s]' "$a"; done`).Output()
		if err != nil {
			// `for a in 'x' exec -- ...` form: ok if the shell accepts it.
			t.Fatalf("sh: %v", err)
		}
		want := "[/opt/my dir/downshift][compact][--]"
		for _, w := range strings.Fields(c) {
			want += "[" + w + "]"
		}
		if string(out) != want {
			t.Fatalf("%q split as %s, want %s", c, out, want)
		}
	}
}

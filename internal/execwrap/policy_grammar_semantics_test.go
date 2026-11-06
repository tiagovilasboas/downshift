// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// A rewritten line must exit as the original does. The stub stands in for the
// downshift binary: it drops `compact --` and runs the argv, so the fake go on
// PATH plays a failing test run and the real shell decides the exit code.
const (
	grammarStubScript = "#!/bin/sh\nshift 2\nexec \"$@\"\n"
	grammarGoScript   = "#!/bin/sh\necho 'FAIL example'\nexit 1\n"
)

// grammarFixture writes the stub and a failing go, and returns the stub path
// and an environment whose PATH finds the fake go first.
func grammarFixture(t *testing.T) (string, []string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("exit semantics are checked with a POSIX sh")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	writeGrammarScript(t, bin, "go", grammarGoScript)
	stub := writeGrammarScript(t, dir, "downshift", grammarStubScript)
	env := append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return stub, env
}

func writeGrammarScript(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// grammarRewrite returns the wrapped line the policy produces for line.
func grammarRewrite(t *testing.T, stub, line string) string {
	t.Helper()
	d := Decide(line, stub)
	if !d.Rewrite {
		t.Fatalf("Decide(%q) not rewritten: %+v", line, d)
	}
	return d.Command
}

// grammarExit runs line under shell and returns its exit code.
func grammarExit(t *testing.T, shell, line string, env []string) int {
	t.Helper()
	cmd := exec.Command(shell, "-c", line)
	cmd.Env = env
	err := cmd.Run()
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		t.Fatalf("run %q under %s: %v", line, shell, err)
	}
	return ee.ExitCode()
}

func TestGrammarExit_ShortCircuitMatchesOriginal(t *testing.T) {
	stub, env := grammarFixture(t)
	const line = "false && go test ./..."
	rewritten := grammarRewrite(t, stub, line)
	orig := grammarExit(t, "sh", line, env)
	got := grammarExit(t, "sh", rewritten, env)
	if orig != 1 || got != orig {
		t.Fatalf("exit of %q = %d, rewritten = %d, want both 1", line, orig, got)
	}
}

func TestGrammarExit_FailingPipelineWithoutPipefail(t *testing.T) {
	stub, env := grammarFixture(t)
	const line = "go test ./... | tail -5"
	rewritten := grammarRewrite(t, stub, line)
	orig := grammarExit(t, "sh", line, env)
	got := grammarExit(t, "sh", rewritten, env)
	if orig != 0 || got != orig {
		t.Fatalf("exit of %q = %d, rewritten = %d, want both 0 (tail decides)", line, orig, got)
	}
}

func TestGrammarExit_FailingPipelineWithPipefail(t *testing.T) {
	stub, env := grammarFixture(t)
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not on PATH: pipefail cannot be checked")
	}
	const line = "go test ./... | tail -5"
	rewritten := grammarRewrite(t, stub, line)
	const prefix = "set -o pipefail; "
	orig := grammarExit(t, bash, prefix+line, env)
	got := grammarExit(t, bash, prefix+rewritten, env)
	if orig != 1 || got != orig {
		t.Fatalf("pipefail exit of %q = %d, rewritten = %d, want both 1", line, orig, got)
	}
}

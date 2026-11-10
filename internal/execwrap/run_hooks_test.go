// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// hookTranscript is what git prints on stderr for a push whose pre-push hook
// ran a clean go test: the hook's lines, the test run, then git's own lines.
var hookTranscript = "┃  test-changed ❯ \n\n[test-changed] ./internal/a ./internal/b ./internal/c ./internal/d\n" + passingSuite +
	"\nsummary: (done in 70.62 seconds)\n✔ test-changed (68.20 seconds)\n" +
	"To github.com:acme/widget.git\n   1a2b3c4..5d6e7f8  feature/cache -> feature/cache\n"

// fakeOnStderr makes name write text on stderr only, exit 0.
func fakeOnStderr(t *testing.T, name, text string) {
	t.Helper()
	src := filepath.Join(t.TempDir(), "err.txt")
	if err := os.WriteFile(src, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	fakeBin(t, name, "cat "+src+" >&2")
}

// git push prints its hooks' output and its own lines on stderr. The go test
// run folds; the hook's lines and git's lines are kept.
func TestRun_GitPushFoldsTheHookTestRun(t *testing.T) {
	fakeOnStderr(t, "git", hookTranscript)
	r, out, errOut := run(t, []string{"git", "push"}, true)
	if !r.Compacted || errOut != "" {
		t.Fatalf("compacted=%v stderr=%q", r.Compacted, errOut)
	}
	for _, want := range []string{"10 packages ok", "✔ test-changed (68.20 seconds)", "To github.com:acme/widget.git", "feature/cache -> feature/cache"} {
		if !strings.Contains(out, want) {
			t.Errorf("lost %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "\t0.1s") {
		t.Errorf("package lines not folded:\n%s", out)
	}
}

// A failed push with nothing to cut comes back as written, each stream as is.
func TestRun_GitPushFailureStaysRaw(t *testing.T) {
	fakeBin(t, "git", "echo 'error: failed to push some refs' >&2; exit 1")
	r, out, errOut := run(t, []string{"git", "push"}, true)
	if r.Compacted || r.ExitCode != 1 || out != "" || errOut != "error: failed to push some refs\n" {
		t.Fatalf("compacted=%v exit=%d stdout=%q stderr=%q", r.Compacted, r.ExitCode, out, errOut)
	}
}

// A push rejected after a clean hook run keeps the rejection and the error
// and cuts the hook's passing package lines.
func TestRun_GitPushRejectedKeepsErrorsCutsNoise(t *testing.T) {
	text := "┃  test ❯ \n\n" + passingSuite + "\nsummary: (done in 4.20 seconds)\n✔ test (4.20 seconds)\n" +
		"To github.com:acme/widget.git\n ! [rejected]        main -> main (non-fast-forward)\n" +
		"error: failed to push some refs to 'github.com:acme/widget.git'\n"
	src := filepath.Join(t.TempDir(), "err.txt")
	if err := os.WriteFile(src, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	fakeBin(t, "git", "cat "+src+" >&2; exit 1")
	r, out, _ := run(t, []string{"git", "push"}, true)
	if !r.Compacted || r.ExitCode != 1 {
		t.Fatalf("compacted=%v exit=%d out=%q", r.Compacted, r.ExitCode, out)
	}
	for _, want := range []string{"[rejected]", "error: failed to push some refs", "✔ test (4.20 seconds)"} {
		if !strings.Contains(out, want) {
			t.Errorf("lost %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "\t0.1s") {
		t.Errorf("package lines kept:\n%s", out)
	}
}

// make check prints its own lines on stdout around a clean go test run.
func TestRun_MakeFoldsTheTestRun(t *testing.T) {
	src := filepath.Join(t.TempDir(), "out.txt")
	if err := os.WriteFile(src, []byte("[1/2] go vet ./...\n[2/2] go test -count=1 ./...\n"+passingSuite), 0o600); err != nil {
		t.Fatal(err)
	}
	fakeBin(t, "make", "cat "+src)
	r, out, _ := run(t, []string{"make", "check"}, true)
	if !r.Compacted || !strings.Contains(out, "[2/2] go test -count=1 ./...") || !strings.Contains(out, "10 packages ok") {
		t.Fatalf("compacted=%v out=%q", r.Compacted, out)
	}
}

// make's stderr is held with its stdout, in write order: a clean run folds
// its go test lines and prints the stderr lines whole, on stderr, after it.
// A terminal on stdin keeps the old rule (gate_stdin.go).
func TestRun_MakeStderrIsHeldAndKept(t *testing.T) {
	src := filepath.Join(t.TempDir(), "out.txt")
	if err := os.WriteFile(src, []byte(passingSuite), 0o600); err != nil {
		t.Fatal(err)
	}
	fakeBin(t, "make", "echo 'listening on :8080' >&2; cat "+src)
	r, out, errOut := run(t, []string{"make", "dev"}, true)
	if !r.Compacted || !strings.Contains(out, "10 packages ok") || errOut != "listening on :8080\n" {
		t.Fatalf("compacted=%v stdout=%q stderr=%q", r.Compacted, out, errOut)
	}
	if r.EmittedBytes != int64(len(out)+len(errOut)) {
		t.Fatalf("emitted=%d, want %d", r.EmittedBytes, len(out)+len(errOut))
	}
}

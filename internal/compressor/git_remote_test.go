// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/compressor"
)

// remoteCase is one synthetic fixture under testdata/remote_runs: the exit
// code of the run, lines that must survive, and lines that must be gone.
type remoteCase struct {
	name string
	exit int
	keep []string
	drop []string
}

var remoteCases = []remoteCase{
	{
		name: "green_hook", exit: 0,
		keep: []string{
			"summary: (done in 21.48 seconds)", "✔️ vet (3.10 seconds)", "✔️ test (18.38 seconds)",
			"To github.com:acme/widget.git", "   1a2b3c4..5d6e7f8  feat/cache -> feat/cache",
			"remote: Resolving deltas: 100% (9/9), completed with 8 local objects.",
		},
		drop: []string{"internal/agents", "[no test files]", "╭", "Counting objects", "Writing objects", "────"},
	},
	{
		name: "failing_hook", exit: 1,
		keep: []string{
			"--- FAIL: TestCheckoutTotal (0.00s)", "    --- FAIL: TestCheckoutTotal/discount_over_100 (0.00s)",
			"        total_test.go:58: got 1999, want 0", "FAIL\tgithub.com/acme/widget/internal/checkout\t0.276s",
			"summary: (done in 9.80 seconds)", "✔️ vet (3.10 seconds)", "🥊 test (6.70 seconds)",
			"error: failed to push some refs to 'github.com:acme/widget.git'",
		},
		drop: []string{"internal/agents", "[no test files]", "╭"},
	},
	{
		name: "non_ff", exit: 1,
		keep: []string{
			" ! [rejected]        main -> main (non-fast-forward)",
			"error: failed to push some refs to 'github.com:acme/widget.git'",
			"hint: Updates were rejected because the tip of your current branch is behind",
			"(skip) vet", "✔️ test (4.20 seconds)",
		},
		drop: []string{"internal/agents", "Compressing objects", "╭"},
	},
	{
		name: "tag", exit: 0,
		keep: []string{
			" * [new tag]         v0.5.4 -> v0.5.4", " + 5d6e7f8...9a8b7c6 main -> main (forced update)",
			"summary: (done in 3.90 seconds)", "✔️ test (3.90 seconds)",
		},
		drop: []string{"internal/agents", "Enumerating objects", "Total 1 (delta 0)"},
	},
}

func TestCompressGitRemoteGolden(t *testing.T) {
	for _, c := range remoteCases {
		t.Run(c.name, func(t *testing.T) {
			in, err := os.ReadFile(filepath.Join("testdata", "remote_runs", c.name+".in"))
			if err != nil {
				t.Fatal(err)
			}
			r := compressor.CompressGitRemote(in, c.exit)
			if !r.Applied || r.Format != compressor.FormatGitRemote || len(r.Output) >= len(in) {
				t.Fatalf("not compacted: applied=%v reason=%q %d -> %d", r.Applied, r.Reason, len(in), len(r.Output))
			}
			t.Logf("%s: %d -> %d bytes", c.name, len(in), len(r.Output))
			golden := filepath.Join("testdata", "remote_runs", c.name+".golden")
			if *updateFailedGolden {
				if err := os.WriteFile(golden, r.Output, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(r.Output, want) {
				t.Fatalf("output differs from golden\n got: %q\nwant: %q", r.Output, want)
			}
			out := string(r.Output)
			for _, k := range c.keep {
				if !strings.Contains(out, k) {
					t.Errorf("lost %q", k)
				}
			}
			for _, d := range c.drop {
				if strings.Contains(out, d) {
					t.Errorf("kept noise %q", d)
				}
			}
		})
	}
}

// Every line the compactor cut is a noise line; nothing else goes missing.
func TestCompressGitRemoteKeepsErrorLines(t *testing.T) {
	for _, c := range remoteCases {
		in, err := os.ReadFile(filepath.Join("testdata", "remote_runs", c.name+".in"))
		if err != nil {
			t.Fatal(err)
		}
		out := compressor.CompressGitRemote(in, c.exit).Output
		for _, line := range strings.Split(string(in), "\n") {
			for _, marker := range []string{"error:", "rejected", "fatal:", "remote:", "FAIL", "hint:", "->"} {
				if strings.Contains(line, marker) && !bytes.Contains(out, []byte(line+"\n")) {
					t.Errorf("%s: lost %q", c.name, line)
				}
			}
		}
	}
}

func TestCompressGitRemoteFailsOpen(t *testing.T) {
	for name, in := range map[string]string{
		"nothing to cut":   "Everything up-to-date\n",
		"no final newline": "To github.com:acme/widget.git\n   1a..2b  main -> main",
		"carriage return":  "Counting objects: 50%\rCounting objects: 100%\r\nTo x\n",
	} {
		r := compressor.CompressGitRemote([]byte(in), 0)
		if r.Applied || string(r.Output) != in {
			t.Errorf("%s: applied=%v output=%q", name, r.Applied, r.Output)
		}
	}
}

// A failing hook can print its failure inside the runner's box ("│ FAIL: ...").
// Only the pure frame lines go; every box line with text after "│" stays, so
// the agent sees why the hook failed, not only "error: failed to push".
func TestCompressGitRemoteKeepsBoxContentOnFailure(t *testing.T) {
	for _, content := range []string{"│ FAIL: lint error", "│ panic: boom", "│ x.go:3: undefined: foo"} {
		in := "Enumerating objects: 5, done.\n" +
			"Counting objects: 100% (5/5), done.\n" +
			"╭──────────────────────────────╮\n" +
			"│                              \n" +
			content + "\n" +
			"╰──────────────────────────────╯\n" +
			"  ────────────────────────────────────\n" +
			"error: failed to push some refs to 'github.com:acme/widget.git'\n"
		r := compressor.CompressGitRemote([]byte(in), 1)
		out := string(r.Output)
		if !strings.Contains(out, content+"\n") {
			t.Errorf("%q: lost the box content line:\n%s", content, out)
		}
		if !strings.Contains(out, "error: failed to push") {
			t.Errorf("%q: lost the error line:\n%s", content, out)
		}
		if !r.Applied || strings.Contains(out, "╭") || strings.Contains(out, "╰") || strings.Contains(out, "────") {
			t.Errorf("%q: frame not cut (applied=%v):\n%s", content, r.Applied, out)
		}
	}
}

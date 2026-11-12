// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package postrun

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/hookport"
)

var allCaps = core.HarnessCapabilities{ReplacesToolOutput: []core.ToolCategory{core.CategoryShell, core.CategorySearch, core.CategoryRead}}

func keepOK(raw []byte) (string, bool) { return "0123456789ab", true }

func bigLog() string {
	var b strings.Builder
	for i := 0; i < 2000; i++ {
		fmt.Fprintf(&b, "step %d of the job finished\n", i)
	}
	return b.String()
}

func TestCompact_Shell(t *testing.T) {
	log := bigLog()
	shell := func(cmd, stdout string) hookport.ToolOutput {
		return hookport.ToolOutput{Category: core.CategoryShell, Command: cmd, Stdout: stdout, HasStdout: true, HasStderr: true, ExitCode: -1}
	}
	opts := Options{Keep: keepOK, Balanced: true}

	res := Compact(allCaps, shell("./job.sh | tee job.log", log), opts)
	if !res.Considered || !res.Replaced || res.Family != "" || res.KeptID == "" || res.EmittedBytes >= res.RawBytes {
		t.Fatalf("piped log: %+v", res)
	}
	if res.Output.Stderr != "" || !res.Output.HasStdout {
		t.Error("the shape flags or stderr changed")
	}

	if res := Compact(allCaps, shell("'/b/downshift' compact -- go test", log), opts); res.Considered {
		t.Error("a wrapped command was compacted again")
	}
	pre := Options{Keep: keepOK, Balanced: true, PreWrapped: func(string) bool { return true }}
	if res := Compact(allCaps, shell("go test ./...", log), pre); res.Considered {
		t.Error("a command the pre-run hook wrapped was compacted again")
	}
	if res := Compact(allCaps, shell("x | y", log), Options{Balanced: true}); res.Replaced || !res.Considered || res.Passthrough == "" {
		t.Errorf("no CCR: %+v, want a raw, logged run", res)
	}
	if res := Compact(allCaps, shell("x | y", log), Options{Keep: keepOK}); res.Replaced {
		t.Error("balanced off still cut")
	}
	op := shell("x | y", log)
	op.Opaque = true
	if res := Compact(allCaps, op, opts); res.Considered {
		t.Error("an opaque result was considered")
	}
	if res := Compact(core.HarnessCapabilities{}, shell("x | y", log), opts); res.Considered {
		t.Error("a harness without the capability was considered")
	}
}

// The marker names the session when the call has a valid hashed one, and is
// the bare `retrieve <id>` otherwise; KeptID stays the bare id.
func TestCompact_HintCarriesSession(t *testing.T) {
	in := hookport.ToolOutput{Category: core.CategoryShell, Command: "./job.sh | tee job.log", Stdout: bigLog(), HasStdout: true, ExitCode: -1}
	for _, tc := range []struct{ session, want string }{
		{"", "downshift retrieve 0123456789ab]"},
		{"not-a-hash", "downshift retrieve 0123456789ab]"},
		{strings.Repeat("ab", 32), "downshift retrieve --session=abababababab 0123456789ab]"},
	} {
		res := Compact(allCaps, in, Options{Keep: keepOK, Balanced: true, Session: tc.session})
		if !res.Replaced || res.KeptID != "0123456789ab" || !strings.Contains(res.Output.Stdout, tc.want) {
			t.Errorf("session %q: replaced=%v kept=%q, want hint %q", tc.session, res.Replaced, res.KeptID, tc.want)
		}
	}
}

func TestCompact_ReadNeverTouched(t *testing.T) {
	in := hookport.ToolOutput{Category: core.CategoryRead, Content: bigLog(), HasContent: true}
	if res := Compact(allCaps, in, Options{Keep: keepOK, Balanced: true}); res.Considered || res.Replaced {
		t.Error("a Read result was compacted")
	}
}

func TestCompact_SearchUnderThresholdKept(t *testing.T) {
	in := hookport.ToolOutput{Category: core.CategorySearch, Paths: []string{"a.go", "b.go"}, HasPaths: true}
	res := Compact(allCaps, in, Options{Keep: keepOK, Balanced: true})
	if res.Replaced || !res.Considered || res.RawBytes != 10 {
		t.Errorf("small search: %+v", res)
	}
}

func TestCompact_StoreFailureLeavesRaw(t *testing.T) {
	fail := func([]byte) (string, bool) { return "", false }
	in := hookport.ToolOutput{Category: core.CategorySearch, Content: strings.Repeat("a.go:1:x err\n", 3000), HasContent: true}
	if res := Compact(allCaps, in, Options{Keep: fail, Balanced: true}); res.Replaced {
		t.Error("cut without stored raw text")
	}
	panicky := func([]byte) (string, bool) { panic("store") }
	if res := Compact(allCaps, in, Options{Keep: panicky, Balanced: true}); res.Replaced {
		t.Error("a panic in the store did not fail open")
	}
}

// A failed command's failure detail and exit code survive the post-run
// compaction: the cut may drop the passing lines, never the failure.
func TestCompact_ShellFailedRunKeepsItsDetailAndCode(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 300; i++ {
		fmt.Fprintf(&b, "ok  \tgithub.com/x/y/pkg%d\t0.01s\n", i)
	}
	b.WriteString("--- FAIL: TestSomething (0.00s)\n    thing_test.go:12: the failure detail\nFAIL\nFAIL\tgithub.com/x/y/bad\t0.01s\n")
	for _, tc := range []struct{ name, stderr string }{{"no stderr", ""}, {"stderr text", "# a note on stderr\n"}} {
		t.Run(tc.name, func(t *testing.T) {
			in := hookport.ToolOutput{Category: core.CategoryShell, Command: "go test ./...", Stdout: b.String(), Stderr: tc.stderr, HasStdout: true, HasStderr: true, ExitCode: 1}
			res := Compact(allCaps, in, Options{Keep: keepOK, Balanced: true})
			if res.ExitCode != 1 {
				t.Errorf("exit code = %d, want 1", res.ExitCode)
			}
			if !strings.Contains(res.Output.Stdout, "the failure detail") {
				t.Errorf("the failure detail is gone from %q", res.Output.Stdout)
			}
			if res.Output.Stderr != tc.stderr {
				t.Errorf("stderr = %q, want it as received %q", res.Output.Stderr, tc.stderr)
			}
			if res.Replaced && res.EmittedBytes >= res.RawBytes {
				t.Errorf("replaced with %d bytes for %d raw", res.EmittedBytes, res.RawBytes)
			}
		})
	}
}

// A small output is left exactly as it was: no marker, no rewrite, and the
// logged row says why.
func TestCompact_ShellSmallOutputStaysRaw(t *testing.T) {
	for _, stdout := range []string{"ok\n", "On branch main\nnothing to commit, working tree clean\n"} {
		in := hookport.ToolOutput{Category: core.CategoryShell, Command: "git status", Stdout: stdout, HasStdout: true, HasStderr: true, ExitCode: -1}
		res := Compact(allCaps, in, Options{Keep: keepOK, Balanced: true})
		if res.Output.Stdout != stdout && res.EmittedBytes >= res.RawBytes {
			t.Errorf("%q: output grew to %q", stdout, res.Output.Stdout)
		}
		if strings.Contains(res.Output.Stdout, "downshift compact") && !res.Replaced {
			t.Errorf("%q: a marker on an output that was not replaced: %q", stdout, res.Output.Stdout)
		}
	}
}

func TestCompact_CompoundCommandNamesItsShape(t *testing.T) {
	shell := func(cmd string) hookport.ToolOutput {
		return hookport.ToolOutput{Category: core.CategoryShell, Command: cmd, Stdout: "a\nb\n", HasStdout: true, ExitCode: 0}
	}
	res := Compact(allCaps, shell("go test ./... | tail -5"), Options{Keep: keepOK, Balanced: true})
	if !res.Considered || res.Inner != "go-test" || res.Kind != "pipe_other" {
		t.Errorf("compound: inner %q kind %q, want go-test pipe_other", res.Inner, res.Kind)
	}
	res = Compact(allCaps, shell("git status"), Options{Keep: keepOK, Balanced: true})
	if res.Inner != "" || res.Kind != "" {
		t.Errorf("simple command: inner %q kind %q, want none", res.Inner, res.Kind)
	}
}

func TestCompact_SearchLedCut(t *testing.T) {
	var b strings.Builder
	for i := 1; i <= 60; i++ {
		fmt.Fprintf(&b, "internal/execwrap/compound_search.go:%d:\tif keep == nil {\n", i)
	}
	stdout := b.String()
	in := hookport.ToolOutput{Category: core.CategoryShell, Command: "grep -rn keep internal | sort", Stdout: stdout, HasStdout: true, ExitCode: -1}
	res := Compact(allCaps, in, Options{Keep: keepOK, Balanced: true})
	if !res.Replaced || !res.Folded || res.KeptID != "0123456789ab" || !strings.Contains(res.Output.Stdout, "  … 35 more in internal/execwrap/compound_search.go\n") {
		t.Fatalf("not cut: replaced %v folded %v id %q\n%s", res.Replaced, res.Folded, res.KeptID, res.Output.Stdout)
	}
	if res := Compact(allCaps, in, Options{Keep: keepOK, Balanced: false}); res.Replaced {
		t.Errorf("cut with the balanced cut off:\n%s", res.Output.Stdout)
	}
	in.Command = "grep -rn keep internal | head -80"
	if res := Compact(allCaps, in, Options{Keep: keepOK, Balanced: true}); res.Replaced {
		t.Errorf("cut within the head count the agent asked for:\n%s", res.Output.Stdout)
	}
	in.Command = "grep -rn keep internal && sed -n 1,60p rows.txt"
	in.Stdout = stdout + strings.Repeat("plain file text line\n", 40)
	if res := Compact(allCaps, in, Options{Keep: keepOK, Balanced: true}); res.Replaced {
		t.Errorf("chained file text cut:\n%s", res.Output.Stdout)
	}
	in.Stdout = stdout
	in.Command = "cat rows.txt | head -80"
	if res := Compact(allCaps, in, Options{Keep: keepOK, Balanced: true}); res.Replaced {
		t.Errorf("file text cut:\n%s", res.Output.Stdout)
	}
}

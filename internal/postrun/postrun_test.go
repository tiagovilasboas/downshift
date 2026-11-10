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

// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/execwrap"
)

// steeringHarnesses are the harnesses with an instructions location, and the
// directory whose existence makes setup find them.
var steeringHarnesses = []struct {
	name, configDir, steeringPath string
}{
	{"codex", ".codex", ".codex/AGENTS.md"},
	{"antigravity", ".gemini", ".gemini/AGENTS.md"},
	{"opencode", ".config/opencode", ".config/opencode/AGENTS.md"},
	{"kirocrew", ".kiro", ".kiro/crew/workspace/.kiro/steering/downshift-compact.md"},
}

func TestSteeringDryRunShowsEveryHarnessAndWritesNothing(t *testing.T) {
	for _, h := range steeringHarnesses {
		t.Run(h.name, func(t *testing.T) {
			e := newSetupEnv(t, h.configDir)
			code, out, _ := e.run("--dry-run", h.name)
			if code != 0 {
				t.Fatalf("exit %d\n%s", code, out)
			}
			if !strings.Contains(out, h.steeringPath) {
				t.Errorf("dry run does not show %s:\n%s", h.steeringPath, out)
			}
			if _, err := os.Stat(e.path(h.steeringPath)); !os.IsNotExist(err) {
				t.Errorf("dry run wrote %s (stat err %v)", h.steeringPath, err)
			}
		})
	}
}

func TestSteeringInstallsEveryHarnessAndIsIdempotent(t *testing.T) {
	for _, h := range steeringHarnesses {
		t.Run(h.name, func(t *testing.T) {
			e := newSetupEnv(t, h.configDir)
			if code, out, _ := e.run("--yes", h.name); code != 0 {
				t.Fatalf("setup exit %d\n%s", code, out)
			}
			first := e.read(h.steeringPath)
			if !strings.Contains(first, "downshift compactor --") || !strings.Contains(first, "Allowlisted families: ") {
				t.Fatalf("steering incomplete:\n%s", first)
			}
			if code, out, _ := e.run("--yes", h.name); code != 0 || !strings.Contains(out, "already up to date") {
				t.Errorf("second run not idempotent (exit %d):\n%s", code, out)
			}
			if second := e.read(h.steeringPath); second != first {
				t.Errorf("second run changed the file")
			}
		})
	}
}

func TestSteeringKiroCrewKeepsFrontmatter(t *testing.T) {
	e := newSetupEnv(t, ".kiro")
	e.run("--yes", "kirocrew")
	if body := e.read(".kiro/crew/workspace/.kiro/steering/downshift-compact.md"); !strings.HasPrefix(body, "---\ninclusion: always\n---\n") {
		t.Errorf("KiroCrew steering lost its frontmatter:\n%s", body)
	}
}

func TestSteeringManagedBlockKeepsUserInstructions(t *testing.T) {
	e := newSetupEnv(t, ".codex")
	e.write(".codex/AGENTS.md", "# My rules\n\nAlways answer in Portuguese.\n")
	if code, out, _ := e.run("--yes", "codex"); code != 0 {
		t.Fatalf("setup exit %d\n%s", code, out)
	}
	body := e.read(".codex/AGENTS.md")
	if !strings.HasPrefix(body, "# My rules\n\nAlways answer in Portuguese.\n\n"+steeringBlockBegin) {
		t.Errorf("user text not kept before the block:\n%s", body)
	}
	if !strings.HasSuffix(body, steeringBlockEnd+"\n") {
		t.Errorf("block not at the end:\n%s", body)
	}
}

func TestSteeringUndoRestoresUserFile(t *testing.T) {
	e := newSetupEnv(t, ".codex")
	const original = "# My rules\n"
	e.write(".codex/AGENTS.md", original)
	if code, out, _ := e.run("--yes", "codex"); code != 0 {
		t.Fatalf("setup exit %d\n%s", code, out)
	}
	if code, out, _ := e.run("--undo", "--yes"); code != 0 {
		t.Fatalf("undo exit %d\n%s", code, out)
	}
	if got := e.read(".codex/AGENTS.md"); got != original {
		t.Errorf("undo left %q, want the original file", got)
	}
}

func TestSteeringUndoRemovesFileSetupCreated(t *testing.T) {
	e := newSetupEnv(t, ".gemini")
	if code, out, _ := e.run("--yes", "antigravity"); code != 0 {
		t.Fatalf("setup exit %d\n%s", code, out)
	}
	if code, out, _ := e.run("--undo", "--yes"); code != 0 {
		t.Fatalf("undo exit %d\n%s", code, out)
	}
	if _, err := os.Stat(e.path(".gemini/AGENTS.md")); !os.IsNotExist(err) {
		t.Errorf("undo should remove the file setup created, stat err=%v", err)
	}
}

func TestMergeSteeringBlockReplacesOnlyTheBlock(t *testing.T) {
	old := steeringBlockBegin + "\nold body\n" + steeringBlockEnd + "\ntail\n"
	got := string(mergeSteeringBlock([]byte("head\n"+old), "new body\n"))
	want := "head\n" + steeringBlockBegin + "\nnew body\n" + steeringBlockEnd + "\ntail\n"
	if got != want {
		t.Errorf("merge = %q, want %q", got, want)
	}
	once := mergeSteeringBlock([]byte("head"), "body\n")
	if twice := mergeSteeringBlock(once, "body\n"); string(twice) != string(once) {
		t.Errorf("merge not idempotent:\n%s\n---\n%s", once, twice)
	}
}

// The steering doc names the allowlisted families in one line. That line must
// be exactly the list FamilyOf accepts, so the doc never drifts from the policy.
func TestSteeringFamilyListMatchesFamilyOf(t *testing.T) {
	const prefix = "Allowlisted families: "
	var line string
	for _, l := range strings.Split(compactSteering, "\n") {
		if strings.HasPrefix(l, prefix) {
			line = strings.TrimPrefix(l, prefix)
		}
	}
	if line == "" {
		t.Fatalf("compact steering has no %q line", prefix)
	}
	if want := execwrap.AllowlistSummary(); line != want {
		t.Errorf("doc list drifted from FamilyOf:\n doc:  %s\n want: %s", line, want)
	}
	for _, form := range strings.Split(line, ", ") {
		if execwrap.FamilyOf(strings.Fields(form)) == execwrap.FamilyNone {
			t.Errorf("doc lists %q, which FamilyOf does not allow", form)
		}
	}
}

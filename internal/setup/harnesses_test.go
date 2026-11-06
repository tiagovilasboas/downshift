// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupDetectionAndNames(t *testing.T) {
	e := newSetupEnv(t, ".codex")
	if code, _, _ := e.run("--yes"); code != 0 {
		t.Fatal("setup failed")
	}
	if _, err := os.Stat(e.path(".claude/settings.json")); err == nil {
		t.Error("undetected harness was written")
	}
	doc := parseDoc(t, e.read(".codex/hooks.json"))
	if got := hookCommands(t, doc, "PreToolUse"); len(got) != 2 || got[0] != testBin+" codex" || got[1] != testBin+" codex" {
		t.Errorf("codex PreToolUse = %v", got)
	}
	if code, _, errOut := e.run("cursor", "--yes"); code == 0 || !strings.Contains(errOut, "~/.cursor does not exist") {
		t.Errorf("named but missing harness: exit %d, %s", code, errOut)
	}
	if code, _, errOut := e.run("nope"); code != 2 || !strings.Contains(errOut, "unknown harness") {
		t.Errorf("unknown harness: exit %d, %s", code, errOut)
	}
	empty := newSetupEnv(t)
	if code, _, errOut := empty.run("--yes"); code == 0 || !strings.Contains(errOut, "no supported harness") {
		t.Errorf("empty home: exit %d, %s", code, errOut)
	}
}

// After writing, setup says in plain words what now applies to the developer:
// who picks subagent models, and the Claude Code session
// policy only when it is really in the file.
func TestSetupSummaryStatesWhatChanges(t *testing.T) {
	e := newSetupEnv(t, ".claude")
	_, out, _ := e.run("claude-code", "--yes", "--you-should-know")
	for _, want := range []string{"What changes for you", "downshift full-power", "plan mode", "opusplan", "You should know is on", "billed", "--no-session-policy", "downshift setup --undo"} {
		if !strings.Contains(out, want) {
			t.Errorf("summary is missing %q:\n%s", want, out)
		}
	}

	// Without the flag the summary must not claim the mod is on.
	_, out, _ = newSetupEnv(t, ".claude").run("claude-code", "--yes")
	if strings.Contains(out, "You should know is on") {
		t.Errorf("summary claims You should know is on without --you-should-know:\n%s", out)
	}

	skipped := newSetupEnv(t, ".claude")
	_, out, _ = skipped.run("claude-code", "--yes", "--no-session-policy")
	if strings.Contains(out, "plan mode") || strings.Contains(out, "opusplan") || strings.Contains(out, "You should know") {
		t.Errorf("summary claims a session policy that was not written:\n%s", out)
	}
	// No control group and no routing variable is mentioned any more.
	t.Setenv("DOWNSHIFT_CONTROL_SHARE", "0.5")
	_, out, _ = newSetupEnv(t, ".claude").run("claude-code", "--yes")
	if strings.Contains(strings.ToLower(out), "control group") || strings.Contains(out, "CONTROL_SHARE") || strings.Contains(out, "1 in ") {
		t.Errorf("the summary must not mention a control group:\n%s", out)
	}

	dry := newSetupEnv(t, ".claude")
	if _, out, _ := dry.run("claude-code", "--dry-run"); strings.Contains(out, "What changes for you") {
		t.Errorf("a dry run wrote nothing, so it must not summarize changes:\n%s", out)
	}

	cursorOnly := newSetupEnv(t, ".cursor")
	if _, out, _ := cursorOnly.run("cursor", "--yes"); strings.Contains(out, "plan mode") {
		t.Errorf("session policy mentioned without Claude Code:\n%s", out)
	}
}

func TestSetupFlatShapes(t *testing.T) {
	e := newSetupEnv(t, ".cursor", ".gemini")
	e.write(".cursor/hooks.json", `{"version":1,"hooks":{"preToolUse":[{"command":"downshift cursor","matcher":"Task"},{"command":"other"}]}}`)
	if code, out, _ := e.run("--yes"); code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	cur := parseDoc(t, e.read(".cursor/hooks.json"))["hooks"].(map[string]any)["preToolUse"].([]any)
	if len(cur) != 3 || cur[0].(map[string]any)["command"] != testBin+" cursor" || cur[1].(map[string]any)["command"] != "other" || cur[2].(map[string]any)["matcher"] != "Shell" {
		t.Errorf("cursor = %v", cur)
	}
	ag := parseDoc(t, e.read(".gemini/config/hooks.json"))["PreToolUse"].([]any)
	if len(ag) != 2 || ag[1].(map[string]any)["matcher"] != "run_command" || ag[0].(map[string]any)["matcher"] != "invoke_subagent" || ag[0].(map[string]any)["command"] != testBin+" antigravity" {
		t.Errorf("antigravity = %v", ag)
	}
}

// Antigravity also accepts hooks inside named groups. A Downshift hook there
// is repointed in place, not duplicated at the root.
func TestSetupAntigravityNamedGroupIsNotDuplicated(t *testing.T) {
	e := newSetupEnv(t, ".gemini")
	e.write(".gemini/config/hooks.json", `{"downshift-model-router":{"enabled":true,"PreToolUse":[{"matcher":"invoke_subagent|spawn_agent","hooks":[{"type":"command","command":"/old/downshift antigravity","timeout":5}]}]},"other-sensor":{"enabled":true,"PreToolUse":[{"matcher":"run_command","hooks":[{"type":"command","command":"/x/sensor"}]}]}}`)
	if code, out, _ := e.run("--yes"); code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	doc := parseDoc(t, e.read(".gemini/config/hooks.json"))
	if _, dup := doc["PreToolUse"]; dup {
		t.Fatalf("duplicate root PreToolUse added: %v", doc["PreToolUse"])
	}
	group := doc["downshift-model-router"].(map[string]any)["PreToolUse"].([]any)[0].(map[string]any)
	if got := group["hooks"].([]any)[0].(map[string]any)["command"]; got != testBin+" antigravity" {
		t.Errorf("command = %v, want repointed to %s antigravity", got, testBin)
	}
	if group["matcher"] != "invoke_subagent|spawn_agent" {
		t.Errorf("operator matcher changed: %v", group["matcher"])
	}
	other := doc["other-sensor"].(map[string]any)["PreToolUse"].([]any)[0].(map[string]any)["hooks"].([]any)[0].(map[string]any)
	if other["command"] != "/x/sensor" {
		t.Errorf("unrelated hook changed: %v", other)
	}
}

// KiroCrew creates its own config. Without it setup writes no hook, but the
// compact steering does not depend on it.
func TestSetupKiroCrewWithoutAConfigInstallsOnlyTheSteering(t *testing.T) {
	e := newSetupEnv(t, ".kiro")
	code, out, _ := e.run("--yes")
	if code != 0 || !strings.Contains(out, "not set up yet") {
		t.Fatalf("exit %d\n%s", code, out)
	}
	if m, _ := filepath.Glob(e.path(".kiro/crew/config.json")); len(m) != 0 {
		t.Errorf("setup created the KiroCrew config: %v", m)
	}
	body, err := os.ReadFile(e.path(".kiro/crew/workspace/.kiro/steering/downshift-compact.md"))
	if err != nil || !strings.Contains(string(body), "downshift compactor --") {
		t.Fatalf("steering not installed: %v\n%s", err, body)
	}
}

// --undo removes a steering setup created (marker-backup path), same as a hook file.
func TestSetupManualHarnessSteeringUndo(t *testing.T) {
	e := newSetupEnv(t, ".kiro")
	if code, out, _ := e.run("--yes"); code != 0 {
		t.Fatalf("setup exit %d\n%s", code, out)
	}
	steer := e.path(".kiro/crew/workspace/.kiro/steering/downshift-compact.md")
	if _, err := os.Stat(steer); err != nil {
		t.Fatalf("steering not installed: %v", err)
	}
	if code, out, _ := e.run("--undo", "--yes"); code != 0 {
		t.Fatalf("undo exit %d\n%s", code, out)
	}
	if _, err := os.Stat(steer); !os.IsNotExist(err) {
		t.Errorf("steering should be removed by undo, stat err=%v", err)
	}
}

func TestSetupBinaryPathWithSpaceIsQuoted(t *testing.T) {
	if got := QuoteCommand("/Applications/My Tools/downshift", "codex"); got != `"/Applications/My Tools/downshift" codex` {
		t.Errorf("QuoteCommand = %s", got)
	}
	for _, c := range []string{`downshift codex`, `/x/downshift codex`, `"/a b/downshift" codex`, `C:\x\downshift.exe codex`} {
		if !IsDownshiftCommand(c, "codex") {
			t.Errorf("%q not recognised", c)
		}
	}
	for _, c := range []string{`notdownshift codex`, `downshift codex --x`, `downshift cursor`, `/x/guard`} {
		if IsDownshiftCommand(c, "codex") {
			t.Errorf("%q wrongly recognised", c)
		}
	}
}

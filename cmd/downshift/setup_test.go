// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testBin = "/opt/ds/downshift"

type setupEnv struct {
	t    *testing.T
	home string
	tick int
}

func newSetupEnv(t *testing.T, dirs ...string) *setupEnv {
	t.Helper()
	home := t.TempDir()
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(home, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return &setupEnv{t: t, home: home}
}

func (e *setupEnv) path(rel string) string { return filepath.Join(e.home, rel) }

func (e *setupEnv) write(rel, content string) {
	e.t.Helper()
	if err := os.MkdirAll(filepath.Dir(e.path(rel)), 0o755); err != nil {
		e.t.Fatal(err)
	}
	if err := os.WriteFile(e.path(rel), []byte(content), 0o644); err != nil {
		e.t.Fatal(err)
	}
}

func (e *setupEnv) read(rel string) string {
	e.t.Helper()
	b, err := os.ReadFile(e.path(rel))
	if err != nil {
		e.t.Fatal(err)
	}
	return string(b)
}

// run executes setup with a fixed binary path and a strictly increasing clock.
func (e *setupEnv) run(args ...string) (code int, out, errOut string) {
	e.t.Helper()
	var o, er bytes.Buffer
	opts := setupOptions{Home: e.home, Binary: testBin, In: strings.NewReader(""), Out: &o, Err: &er}
	opts.Now = func() time.Time {
		e.tick++
		return time.Date(2026, 10, 13, 12, 0, 0, e.tick*1000, time.UTC)
	}
	for _, a := range args {
		switch a {
		case "--dry-run":
			opts.DryRun = true
		case "--undo":
			opts.Undo = true
		case "--yes":
			opts.Yes = true
		case "--no-session-policy":
			opts.NoSessionPolicy = true
		default:
			opts.Names = append(opts.Names, a)
		}
	}
	return setupRun(opts), o.String(), er.String()
}

func parseDoc(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, s)
	}
	return m
}

// hookCommands lists every command under hooks[event] of a nested-shape file.
func hookCommands(t *testing.T, doc map[string]any, event string) []string {
	t.Helper()
	var cmds []string
	hooks, _ := doc["hooks"].(map[string]any)
	groups, _ := hooks[event].([]any)
	for _, g := range groups {
		for _, h := range g.(map[string]any)["hooks"].([]any) {
			cmds = append(cmds, h.(map[string]any)["command"].(string))
		}
	}
	return cmds
}

func TestSetupFreshClaudeCode(t *testing.T) {
	e := newSetupEnv(t, ".claude")
	code, out, _ := e.run("--yes")
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	doc := parseDoc(t, e.read(".claude/settings.json"))
	want := map[string]string{
		"PreToolUse":         testBin + " claude-code",
		"PostToolUse":        testBin + " claude-code-post-tool-use",
		"PostToolUseFailure": testBin + " claude-code-post-tool-use",
		"SubagentStop":       testBin + " claude-code-subagent-stop",
	}
	for event, cmd := range want {
		got := hookCommands(t, doc, event)
		if len(got) != 1 || got[0] != cmd {
			t.Errorf("%s = %v, want [%s]", event, got, cmd)
		}
	}
	pre := doc["hooks"].(map[string]any)["PreToolUse"].([]any)[0].(map[string]any)
	if pre["matcher"] != "Agent|Task" {
		t.Errorf("matcher = %v", pre["matcher"])
	}
	sl := doc["statusLine"].(map[string]any)
	if sl["type"] != "command" || sl["command"] != testBin+" quota claude-statusline" {
		t.Errorf("statusLine = %v", sl)
	}
}

func TestSetupKeepsUnrelatedKeysAndHooks(t *testing.T) {
	e := newSetupEnv(t, ".claude")
	e.write(".claude/settings.json", `{
  "model": "opus",
  "permissions": {"allow": ["Bash(ls:*)"]},
  "hooks": {
    "PreToolUse": [
      {"matcher": "Bash", "hooks": [{"type": "command", "command": "/usr/local/bin/guard"}]}
    ],
    "Stop": [{"hooks": [{"type": "command", "command": "notify"}]}]
  },
  "env": {"A": "<b>&"}
}`)
	if code, out, _ := e.run("--yes"); code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	raw := e.read(".claude/settings.json")
	doc := parseDoc(t, raw)
	if doc["model"] != "opus" || doc["permissions"].(map[string]any)["allow"].([]any)[0] != "Bash(ls:*)" {
		t.Errorf("unrelated keys lost: %s", raw)
	}
	if !strings.Contains(raw, `"<b>&"`) {
		t.Errorf("HTML characters were escaped: %s", raw)
	}
	if got := hookCommands(t, doc, "Stop"); len(got) != 1 || got[0] != "notify" {
		t.Errorf("Stop hook changed: %v", got)
	}
	got := hookCommands(t, doc, "PreToolUse")
	if len(got) != 2 || got[0] != "/usr/local/bin/guard" || got[1] != testBin+" claude-code" {
		t.Errorf("PreToolUse = %v", got)
	}
	if strings.Index(raw, `"model"`) > strings.Index(raw, `"hooks"`) {
		t.Errorf("key order changed: %s", raw)
	}
}

func TestSetupIdempotent(t *testing.T) {
	e := newSetupEnv(t, ".claude", ".cursor", ".codex", ".gemini")
	if code, out, _ := e.run("--yes"); code != 0 {
		t.Fatalf("first run exit %d\n%s", code, out)
	}
	files := []string{".claude/settings.json", ".cursor/hooks.json", ".codex/hooks.json", ".gemini/config/hooks.json"}
	first := map[string]string{}
	for _, f := range files {
		first[f] = e.read(f)
	}
	baks, _ := filepath.Glob(e.path(".claude/*"))
	code, out, _ := e.run("--yes")
	if code != 0 || !strings.Contains(out, "already up to date") || strings.Contains(out, "@@") {
		t.Fatalf("second run should be a no-op, exit %d\n%s", code, out)
	}
	for _, f := range files {
		if e.read(f) != first[f] {
			t.Errorf("%s changed on second run", f)
		}
	}
	if again, _ := filepath.Glob(e.path(".claude/*")); len(again) != len(baks) {
		t.Errorf("second run wrote a backup: %v -> %v", baks, again)
	}
}

func TestSetupReplacesStaleBinaryAndOldMatcher(t *testing.T) {
	e := newSetupEnv(t, ".claude")
	e.write(".claude/settings.json", `{"hooks":{"PreToolUse":[
 {"matcher":"Task","hooks":[{"type":"command","command":"downshift claude-code"}]}],
 "SubagentStop":[{"hooks":[{"type":"command","command":"\"/old path/downshift\" claude-code-subagent-stop"}]}]}}`)
	if code, out, _ := e.run("--yes"); code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	doc := parseDoc(t, e.read(".claude/settings.json"))
	if got := hookCommands(t, doc, "PreToolUse"); len(got) != 1 || got[0] != testBin+" claude-code" {
		t.Errorf("PreToolUse = %v", got)
	}
	if m := doc["hooks"].(map[string]any)["PreToolUse"].([]any)[0].(map[string]any)["matcher"]; m != "Agent|Task" {
		t.Errorf("matcher = %v", m)
	}
	if got := hookCommands(t, doc, "SubagentStop"); len(got) != 1 || got[0] != testBin+" claude-code-subagent-stop" {
		t.Errorf("SubagentStop = %v", got)
	}
}

func TestSetupLeavesExistingStatusLine(t *testing.T) {
	e := newSetupEnv(t, ".claude")
	e.write(".claude/settings.json", `{"statusLine":{"type":"command","command":"~/bin/mystatus.sh"}}`)
	code, out, _ := e.run("--yes")
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	sl := parseDoc(t, e.read(".claude/settings.json"))["statusLine"].(map[string]any)
	if sl["command"] != "~/bin/mystatus.sh" {
		t.Errorf("statusLine overwritten: %v", sl)
	}
	if !strings.Contains(out, "quota import --harness claude-code --source claude-statusline") {
		t.Errorf("no chaining hint:\n%s", out)
	}
}

func TestSetupRepointsOwnStatusLine(t *testing.T) {
	e := newSetupEnv(t, ".claude")
	e.write(".claude/settings.json", `{"statusLine":{"type":"command","command":"downshift quota claude-statusline"}}`)
	if code, out, _ := e.run("--yes"); code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	sl := parseDoc(t, e.read(".claude/settings.json"))["statusLine"].(map[string]any)
	if sl["command"] != testBin+" quota claude-statusline" {
		t.Errorf("statusLine = %v", sl)
	}
}

func TestSetupDryRunWritesNothing(t *testing.T) {
	e := newSetupEnv(t, ".claude", ".cursor")
	e.write(".claude/settings.json", `{"model":"opus"}`)
	code, out, _ := e.run("--dry-run")
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	if e.read(".claude/settings.json") != `{"model":"opus"}` {
		t.Error("existing file modified")
	}
	if _, err := os.Stat(e.path(".cursor/hooks.json")); err == nil {
		t.Error("new file created")
	}
	for _, want := range []string{"would change", "+++ b/~/.claude/settings.json", "\"matcher\": \"Agent|Task\"", "Dry run: nothing was written."} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if m, _ := filepath.Glob(e.path(".claude/*downshift*")); len(m) != 0 {
		t.Errorf("backup written in dry run: %v", m)
	}
}

func TestSetupUndoRestoresLatestBackup(t *testing.T) {
	e := newSetupEnv(t, ".claude", ".cursor")
	original := "{\n  \"model\": \"opus\"\n}\n"
	e.write(".claude/settings.json", original)
	if code, out, _ := e.run("--yes"); code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	if e.read(".claude/settings.json") == original {
		t.Fatal("setup changed nothing")
	}
	if m, _ := filepath.Glob(e.path(".claude/settings.json.downshift-bak-*")); len(m) != 1 {
		t.Fatalf("want one backup, got %v", m)
	}
	// A dry-run undo changes nothing.
	if code, _, _ := e.run("--undo", "--dry-run"); code != 0 || e.read(".claude/settings.json") == original {
		t.Fatal("dry-run undo modified the file")
	}
	code, out, _ := e.run("--undo", "--yes")
	if code != 0 {
		t.Fatalf("undo exit %d\n%s", code, out)
	}
	if e.read(".claude/settings.json") != original {
		t.Errorf("not restored:\n%s", e.read(".claude/settings.json"))
	}
	// The cursor file did not exist before setup: undo removes it.
	if _, err := os.Stat(e.path(".cursor/hooks.json")); err == nil {
		t.Error("file created by setup survived undo")
	}
}

func TestSetupUndoPicksNewestBackup(t *testing.T) {
	e := newSetupEnv(t, ".claude")
	e.write(".claude/settings.json", `{"a":1}`)
	e.run("--yes")
	// Simulate the user changing the file, then running setup with another binary path.
	e.write(".claude/settings.json", `{"a":2}`)
	e.run("--yes")
	if code, _, _ := e.run("--undo", "--yes"); code != 0 {
		t.Fatal("undo failed")
	}
	if got := e.read(".claude/settings.json"); got != `{"a":2}` {
		t.Errorf("restored %q, want the newest backup", got)
	}
}

func TestSetupMalformedJSONAbortsWithoutWriting(t *testing.T) {
	e := newSetupEnv(t, ".claude", ".cursor")
	e.write(".claude/settings.json", `{"hooks": {`)
	code, out, errOut := e.run("--yes")
	if code == 0 {
		t.Fatalf("want non-zero exit\n%s", out)
	}
	if e.read(".claude/settings.json") != `{"hooks": {` {
		t.Error("malformed file was modified")
	}
	if m, _ := filepath.Glob(e.path(".claude/*downshift*")); len(m) != 0 {
		t.Errorf("backup or temp written: %v", m)
	}
	if !strings.Contains(errOut, "not valid JSON") {
		t.Errorf("stderr: %s", errOut)
	}
	// The healthy harness is still wired.
	if _, err := os.Stat(e.path(".cursor/hooks.json")); err != nil {
		t.Errorf("cursor not wired: %v", err)
	}
}

func TestSetupWrongShapeAborts(t *testing.T) {
	e := newSetupEnv(t, ".claude")
	e.write(".claude/settings.json", `{"hooks":{"PreToolUse":"nope"}}`)
	if code, _, errOut := e.run("--yes"); code == 0 || !strings.Contains(errOut, "not a JSON array") {
		t.Fatalf("exit %d, stderr %s", code, errOut)
	}
	if e.read(".claude/settings.json") != `{"hooks":{"PreToolUse":"nope"}}` {
		t.Error("file modified")
	}
}

func TestSetupDetectionAndNames(t *testing.T) {
	e := newSetupEnv(t, ".codex")
	if code, _, _ := e.run("--yes"); code != 0 {
		t.Fatal("setup failed")
	}
	if _, err := os.Stat(e.path(".claude/settings.json")); err == nil {
		t.Error("undetected harness was written")
	}
	doc := parseDoc(t, e.read(".codex/hooks.json"))
	if got := hookCommands(t, doc, "PreToolUse"); len(got) != 1 || got[0] != testBin+" codex" {
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

// Claude Code starts in plan mode with opusplan only for keys the user has not
// set; anything already set is kept and setup says how to switch.
func TestSetupClaudeCodeMainSessionPolicy(t *testing.T) {
	policy := func(e *setupEnv) (model any, mode any) {
		doc := parseDoc(t, e.read(".claude/settings.json"))
		perms, _ := doc["permissions"].(map[string]any)
		return doc["model"], perms["defaultMode"]
	}

	fresh := newSetupEnv(t, ".claude")
	if code, out, _ := fresh.run("claude-code", "--yes"); code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	if m, d := policy(fresh); m != "opusplan" || d != "plan" {
		t.Errorf("fresh = model %v, defaultMode %v; want opusplan, plan", m, d)
	}
	if w := parseDoc(t, fresh.read(".claude/settings.json"))["autoCompactWindow"]; w != float64(400000) {
		t.Errorf("fresh autoCompactWindow = %v (%T); want the plain integer 400000", w, w)
	}

	pinned := newSetupEnv(t, ".claude")
	pinned.write(".claude/settings.json", `{"model":"claude-sonnet-5-5","autoCompactWindow":250000,"permissions":{"allow":["Bash(ls:*)"],"defaultMode":"acceptEdits"}}`)
	code, out, _ := pinned.run("claude-code", "--yes")
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	if m, d := policy(pinned); m != "claude-sonnet-5-5" || d != "acceptEdits" {
		t.Errorf("pinned = model %v, defaultMode %v; want both kept", m, d)
	}
	if !strings.Contains(out, `"model": "opusplan"`) || !strings.Contains(out, "permissions.defaultMode") || !strings.Contains(out, "autoCompactWindow") {
		t.Errorf("no notes for the kept values:\n%s", out)
	}
	if w := parseDoc(t, pinned.read(".claude/settings.json"))["autoCompactWindow"]; w != float64(250000) {
		t.Errorf("pinned autoCompactWindow = %v; want 250000 kept", w)
	}
	if perms := parseDoc(t, pinned.read(".claude/settings.json"))["permissions"].(map[string]any); len(perms["allow"].([]any)) != 1 {
		t.Errorf("existing permissions rules changed: %v", perms)
	}

	already := newSetupEnv(t, ".claude")
	already.write(".claude/settings.json", `{"model":"opusplan","autoCompactWindow":400000,"permissions":{"defaultMode":"plan"}}`)
	if _, out, _ := already.run("claude-code", "--yes"); strings.Contains(out, "keeps its pinned") || strings.Contains(out, "keep their") {
		t.Errorf("note printed although the policy is already set:\n%s", out)
	}

	notObject := newSetupEnv(t, ".claude")
	notObject.write(".claude/settings.json", `{"permissions":"strict"}`)
	if code, out, errOut := notObject.run("claude-code", "--yes"); code == 0 || !strings.Contains(out+errOut, `"permissions" is not a JSON object`) {
		t.Errorf("non-object permissions: exit %d, want an error naming it\n%s%s", code, out, errOut)
	}
	if got := parseDoc(t, notObject.read(".claude/settings.json"))["permissions"]; got != "strict" {
		t.Errorf("non-object permissions overwritten: %v", got)
	}
}

// After writing, setup says in plain words what now applies to the developer:
// who picks subagent models, the control group, and the Claude Code session
// policy only when it is really in the file.
func TestSetupSummaryStatesWhatChanges(t *testing.T) {
	t.Setenv("DOWNSHIFT_CONTROL_SHARE", "")
	e := newSetupEnv(t, ".claude")
	_, out, _ := e.run("claude-code", "--yes")
	for _, want := range []string{"What changes for you", "1 in 10", "downshift report", "plan mode", "opusplan", "400k tokens", "--no-session-policy", "downshift setup --undo"} {
		if !strings.Contains(out, want) {
			t.Errorf("summary is missing %q:\n%s", want, out)
		}
	}

	skipped := newSetupEnv(t, ".claude")
	_, out, _ = skipped.run("claude-code", "--yes", "--no-session-policy")
	if strings.Contains(out, "plan mode") || strings.Contains(out, "opusplan") || strings.Contains(out, "400k tokens") {
		t.Errorf("summary claims a session policy that was not written:\n%s", out)
	}
	if !strings.Contains(out, "1 in 10") {
		t.Errorf("control group missing from the summary:\n%s", out)
	}

	t.Setenv("DOWNSHIFT_CONTROL_SHARE", "0")
	off := newSetupEnv(t, ".claude")
	_, out, _ = off.run("claude-code", "--yes")
	if strings.Contains(out, "1 in 10") || !strings.Contains(out, "off") {
		t.Errorf("control group off should be said as off:\n%s", out)
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

// --no-session-policy wires the hooks and leaves the main-session policy alone.
func TestSetupNoSessionPolicySkipsThePolicy(t *testing.T) {
	e := newSetupEnv(t, ".claude")
	if code, out, _ := e.run("claude-code", "--yes", "--no-session-policy"); code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	doc := parseDoc(t, e.read(".claude/settings.json"))
	if _, ok := doc["model"]; ok {
		t.Errorf("model written with --no-session-policy: %v", doc["model"])
	}
	if _, ok := doc["permissions"]; ok {
		t.Errorf("permissions written with --no-session-policy: %v", doc["permissions"])
	}
	if _, ok := doc["hooks"].(map[string]any)["PreToolUse"]; !ok {
		t.Errorf("hooks not wired: %v", doc["hooks"])
	}
}

func TestSetupFlatShapes(t *testing.T) {
	e := newSetupEnv(t, ".cursor", ".gemini")
	e.write(".cursor/hooks.json", `{"version":1,"hooks":{"preToolUse":[{"command":"downshift cursor","matcher":"Task"},{"command":"other"}]}}`)
	if code, out, _ := e.run("--yes"); code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	cur := parseDoc(t, e.read(".cursor/hooks.json"))["hooks"].(map[string]any)["preToolUse"].([]any)
	if len(cur) != 2 || cur[0].(map[string]any)["command"] != testBin+" cursor" || cur[1].(map[string]any)["command"] != "other" {
		t.Errorf("cursor = %v", cur)
	}
	ag := parseDoc(t, e.read(".gemini/config/hooks.json"))["PreToolUse"].([]any)
	if len(ag) != 1 || ag[0].(map[string]any)["matcher"] != "invoke_subagent" || ag[0].(map[string]any)["command"] != testBin+" antigravity" {
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

func TestSetupManualHarnessIsReportedNotWritten(t *testing.T) {
	e := newSetupEnv(t, ".kiro")
	code, out, _ := e.run("--yes")
	if code != 0 || !strings.Contains(out, "kirocrew-pre-tool-use.sh") {
		t.Fatalf("exit %d\n%s", code, out)
	}
	if m, _ := filepath.Glob(e.path(".kiro/*")); len(m) != 0 {
		t.Errorf("kiro dir modified: %v", m)
	}
}

func TestSetupBinaryPathWithSpaceIsQuoted(t *testing.T) {
	if got := quoteCommand("/Applications/My Tools/downshift", "codex"); got != `"/Applications/My Tools/downshift" codex` {
		t.Errorf("quoteCommand = %s", got)
	}
	for _, c := range []string{`downshift codex`, `/x/downshift codex`, `"/a b/downshift" codex`, `C:\x\downshift.exe codex`} {
		if !isDownshiftCommand(c, "codex") {
			t.Errorf("%q not recognised", c)
		}
	}
	for _, c := range []string{`notdownshift codex`, `downshift codex --x`, `downshift cursor`, `/x/guard`} {
		if isDownshiftCommand(c, "codex") {
			t.Errorf("%q wrongly recognised", c)
		}
	}
}

func TestSetupDeclinedPromptWritesNothing(t *testing.T) {
	e := newSetupEnv(t, ".codex")
	var o, er bytes.Buffer
	code := setupRun(setupOptions{Home: e.home, Binary: testBin, In: strings.NewReader("n\n"), Out: &o, Err: &er, Now: time.Now})
	if code == 0 {
		t.Error("declined prompt should not exit 0")
	}
	if _, err := os.Stat(e.path(".codex/hooks.json")); err == nil {
		t.Error("written without consent")
	}
}

// When the plugin already carries the hooks, setup says so before writing.
func TestPlanTarget_NotesEnabledPlugin(t *testing.T) {
	home := t.TempDir()
	var target setupTarget
	for _, st := range setupTargets {
		if st.Plugin != "" {
			target = st
			break
		}
	}
	if target.Plugin == "" {
		t.Skip("no target declares a plugin")
	}
	path := filepath.Join(home, filepath.FromSlash(target.File))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"enabledPlugins":{"`+target.Plugin+`":true}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	p := planTarget(setupOptions{Home: home, Binary: "/usr/local/bin/downshift"}, target)
	if p.err != nil {
		t.Fatal(p.err)
	}
	if len(p.notes) == 0 || !strings.Contains(p.notes[0], "none were written here") {
		t.Fatalf("notes = %v", p.notes)
	}
	if strings.Contains(string(p.after), `"hooks"`) || strings.Contains(string(p.after), "downshift claude-code") {
		t.Fatalf("hooks must not be written while the plugin is enabled:\n%s", p.after)
	}
	if err := os.WriteFile(path, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	p = planTarget(setupOptions{Home: home, Binary: "/usr/local/bin/downshift"}, target)
	for _, n := range p.notes {
		if strings.Contains(n, "none were written here") {
			t.Fatal("no plugin, no note")
		}
	}
	if !strings.Contains(string(p.after), `"hooks"`) {
		t.Fatalf("without the plugin the hooks are written:\n%s", p.after)
	}
}

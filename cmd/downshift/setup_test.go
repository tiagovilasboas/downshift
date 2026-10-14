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

// Claude Code gets opusplan as the main-session policy only when no model is
// pinned; a pinned model is kept and setup says how to switch.
func TestSetupClaudeCodeOpusplanPolicy(t *testing.T) {
	fresh := newSetupEnv(t, ".claude")
	if code, out, _ := fresh.run("claude-code", "--yes"); code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	if got := parseDoc(t, fresh.read(".claude/settings.json"))["model"]; got != "opusplan" {
		t.Errorf("fresh model = %v, want opusplan", got)
	}

	pinned := newSetupEnv(t, ".claude")
	pinned.write(".claude/settings.json", `{"model":"claude-sonnet-5-5"}`)
	code, out, _ := pinned.run("claude-code", "--yes")
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	if got := parseDoc(t, pinned.read(".claude/settings.json"))["model"]; got != "claude-sonnet-5-5" {
		t.Errorf("pinned model = %v, want it kept", got)
	}
	if !strings.Contains(out, `"model": "opusplan"`) {
		t.Errorf("no opusplan note for a pinned model:\n%s", out)
	}

	already := newSetupEnv(t, ".claude")
	already.write(".claude/settings.json", `{"model":"opusplan"}`)
	if _, out, _ := already.run("claude-code", "--yes"); strings.Contains(out, "keeps its pinned") {
		t.Errorf("note printed although opusplan is already set:\n%s", out)
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

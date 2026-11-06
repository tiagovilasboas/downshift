// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/hooks"
	"github.com/tiagovilasboas/downshift/internal/setup/configfile"
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
	withOwnerAppState(t, OwnerAppStopped) // never read this machine's processes
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
	opts := Options{Home: e.home, Binary: testBin, SkipDiscovery: true, In: strings.NewReader(""), Out: &o, Err: &er}
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
		case "--you-should-know":
			opts.YouShouldKnow = true
		case "--no-session-policy":
			opts.NoSessionPolicy = true
		case "--no-mods":
			opts.NoMods = true
		case "--mcp":
			opts.Mcp = true
		default:
			opts.Names = append(opts.Names, a)
		}
	}
	return Run(opts), o.String(), er.String()
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
		if event == "PreToolUse" || event == "PostToolUse" {
			// Agent|Task spawns and Bash (the exec wrapper) share the handler;
			// PostToolUse adds Read|Grep|Glob for the main-session work count.
			n := 2
			if event == "PostToolUse" {
				n = 3
			}
			if len(got) != n {
				t.Errorf("%s = %v, want %d x %s", event, got, n, cmd)
			}
			for _, g := range got {
				if g != cmd {
					t.Errorf("%s = %v, want only %s", event, got, cmd)
				}
			}
			continue
		}
		if len(got) != 1 || got[0] != cmd {
			t.Errorf("%s = %v, want [%s]", event, got, cmd)
		}
	}
	if !hasMatcher(doc, "PreToolUse", "Agent|Task") || !hasMatcher(doc, "PreToolUse", "Bash") {
		t.Errorf("PreToolUse must carry the Agent|Task and Bash matchers: %v", doc["hooks"].(map[string]any)["PreToolUse"])
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
	if len(got) != 3 || got[0] != "/usr/local/bin/guard" || got[1] != testBin+" claude-code" || got[2] != testBin+" claude-code" {
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
	if got := hookCommands(t, doc, "PreToolUse"); len(got) != 2 || got[0] != testBin+" claude-code" {
		t.Errorf("PreToolUse = %v", got)
	}
	if !hasMatcher(doc, "PreToolUse", "Agent|Task") {
		t.Errorf("PreToolUse lost its Agent|Task matcher")
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
func TestSetupOpenCodeWritesTheBridge(t *testing.T) {
	e := newSetupEnv(t, ".config/opencode")
	code, out, _ := e.run("--yes")
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	if strings.Contains(out, "not wired by setup") {
		t.Fatalf("opencode must be wired, not reported as manual:\n%s", out)
	}
	body := e.read(".config/opencode/plugins/downshift/index.ts")
	for _, want := range []string{
		configfile.OwnedPhrase,
		`downshift opencode`,
		`"honor"`,
		`"end"`,
		`ctx.session.switchModel`,
		`ctx.shell.hook("create.before"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("installed bridge lacks %q", want)
		}
	}
	for _, m := range hooks.OpenCodeModules {
		if !strings.Contains(e.read(".config/opencode/plugins/downshift/"+path.Base(m)), configfile.OwnedPhrase) {
			t.Errorf("module %s missing or without the ownership marker", m)
		}
	}
	if !strings.Contains(e.read(".config/opencode/AGENTS.md"), SteeringBlockBegin) {
		t.Errorf("compact steering block missing from opencode AGENTS.md")
	}
}

// A bridge file a person wrote is never overwritten: the ownership marker is
// the claim.
func TestSetupOpenCodeLeavesAForeignBridgeAlone(t *testing.T) {
	e := newSetupEnv(t, ".config/opencode")
	e.write(".config/opencode/plugins/downshift/index.ts", "// my own plugin\n")
	code, out, _ := e.run("--yes")
	if code == 0 {
		t.Fatalf("a foreign bridge must fail the run, not be replaced:\n%s", out)
	}
	if got := e.read(".config/opencode/plugins/downshift/index.ts"); got != "// my own plugin\n" {
		t.Fatalf("foreign bridge was rewritten: %q", got)
	}
}

// Undo removes the bridge setup wrote and the folders it created, and puts a
// previously missing file back to missing.
func TestSetupUndoRemovesTheOpenCodeBridge(t *testing.T) {
	e := newSetupEnv(t, ".config/opencode")
	if code, out, _ := e.run("--yes"); code != 0 {
		t.Fatalf("setup exit %d\n%s", code, out)
	}
	if _, err := os.Stat(e.path(".config/opencode/plugins/downshift/index.ts")); err != nil {
		t.Fatalf("bridge not installed: %v", err)
	}
	if code, out, _ := e.run("--undo", "--yes"); code != 0 {
		t.Fatalf("undo exit %d\n%s", code, out)
	}
	if _, err := os.Stat(e.path(".config/opencode/plugins/downshift/index.ts")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("bridge survived undo: %v", err)
	}
	for _, m := range hooks.OpenCodeModules {
		if _, err := os.Stat(e.path(".config/opencode/plugins/downshift/" + path.Base(m))); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("module %s survived undo: %v", m, err)
		}
	}
	if _, err := os.Stat(e.path(".config/opencode/plugins")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("empty plugins folder survived undo: %v", err)
	}
}

// --undo removes a steering setup created (marker-backup path), same as a hook file.

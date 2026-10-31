// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/paths"
)

// doctorFixture is a temp HOME with a downshift binary on a fake PATH.
type doctorFixture struct {
	home    string
	bin     string // the binary "on PATH"
	env     doctorEnv
	started time.Time
}

func newDoctorFixture(t *testing.T) *doctorFixture {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, k := range []string{paths.EnvStateDir, "DOWNSHIFT_EVENT_LOG", "DOWNSHIFT_SESSION_MODELS", "DOWNSHIFT_DISCOVERED", "DOWNSHIFT_CURSOR_NATIVE_FILE", "CLAUDE_CODE_SUBAGENT_MODEL", "DOWNSHIFT_QUOTA_FILE", "DOWNSHIFT_QUOTA_MODE"} {
		t.Setenv(k, "")
	}
	bin := filepath.Join(home, "bin", "downshift")
	writeFile(t, bin, "#!/bin/sh\n")
	if err := os.Chmod(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	// The MCP check reads the real ~/.claude.json and spawns a server, which a
	// buildDoctorReport fixture does not control; stub it off here. The MCP
	// check has its own tests (doctor_mcp_test.go) and a wiring test below.
	stubDoctorMcpCheck(t)
	f := &doctorFixture{home: home, bin: bin, started: time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)}
	f.env = doctorEnv{
		Home:       home,
		Executable: func() (string, error) { return bin, nil },
		LookPath:   func(string) (string, error) { return bin, nil },
		Now:        func() time.Time { return f.started },
		Getenv:     func(string) string { return "" },
		Cat:        cmdCat,
	}
	return f
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// claudeSettings is a fixture settings.json with every expected hook, each
// running command prefix.
func claudeSettings(prefix string) string {
	hook := func(sub string) string {
		return fmt.Sprintf(`[{"matcher":"Agent|Task","hooks":[{"type":"command","command":"%s %s"}]}]`, prefix, sub)
	}
	return fmt.Sprintf(`{"hooks":{"PreToolUse":%s,"PostToolUse":%s,"PostToolUseFailure":%s,
"SessionStart":[{"hooks":[{"type":"command","command":"%s claude-code-session-start"}]}],
"SubagentStop":[{"hooks":[{"type":"command","command":"%s claude-code-subagent-stop"}]}],
"UserPromptSubmit":[{"hooks":[{"type":"command","command":"%s dci hook claude-code"}]}]}}`,
		hook("claude-code"), hook("claude-code-post-tool-use"), hook("claude-code-post-tool-use"), prefix, prefix, prefix)
}

// eventLines writes a fresh claude-code shift that a later spawn confirms.
func (f *doctorFixture) writeEvents(t *testing.T, dir string, age time.Duration, withHonor bool) {
	t.Helper()
	ts := f.started.Add(-age)
	lines := []string{
		fmt.Sprintf(`{"correlation_id":"a1","timestamp":%q,"harness":"claude-code","session_id":"s1","requested_model":"opus","final_model":"sonnet","outcome":"rewrite_emitted"}`, ts.Format(time.RFC3339Nano)),
	}
	if withHonor {
		lines = append(lines, fmt.Sprintf(`{"correlation_id":"a2","timestamp":%q,"harness":"claude-code","session_id":"s1","requested_model":"sonnet","final_model":"sonnet","outcome":"allow"}`, ts.Add(time.Second).Format(time.RFC3339Nano)))
	}
	writeFile(t, filepath.Join(dir, "events.jsonl"), strings.Join(lines, "\n")+"\n")
}

func (f *doctorFixture) check(t *testing.T, rep *doctorReport, harness, id string) doctorCheck {
	t.Helper()
	for _, c := range rep.Checks {
		if c.Harness == harness && c.ID == id {
			return c
		}
	}
	t.Fatalf("no check %s/%s in %+v", harness, id, rep.Checks)
	return doctorCheck{}
}

func wantStatus(t *testing.T, c doctorCheck, status string) {
	t.Helper()
	if c.Status != status {
		t.Errorf("%s/%s = %s (%s), want %s", c.Harness, c.ID, c.Status, c.Detail, status)
	}
	if status != doctorOK && c.Fix == "" {
		t.Errorf("%s/%s is %s but has no fix", c.Harness, c.ID, c.Status)
	}
}

func TestDoctor_AllGood(t *testing.T) {
	f := newDoctorFixture(t)
	useSessionList(t, "claude-code", "claude-haiku-4-5", "claude-sonnet-4-5", "claude-opus-4-5")
	writeFile(t, filepath.Join(f.home, ".claude", "settings.json"), claudeSettings(f.bin))
	f.writeEvents(t, filepath.Join(f.home, ".downshift"), time.Hour, true)
	if err := os.MkdirAll(filepath.Join(f.home, ".downshift"), 0o755); err != nil {
		t.Fatal(err)
	}

	rep := buildDoctorReport(f.env)
	if rep.Failed != 0 || rep.Warnings != 0 {
		var b bytes.Buffer
		rep.writeText(&b, false, false)
		t.Fatalf("want a clean report:\n%s", b.String())
	}
	for _, id := range []string{"hooks wired", "hook binary", "session list", "last hook event", "rewrite honored"} {
		wantStatus(t, f.check(t, rep, "claude-code", id), doctorOK)
	}
	if len(rep.NotDetected) == 0 || rep.NotDetected[0] == "claude-code" {
		t.Errorf("harnesses without a hook file must be skipped, got %v", rep.NotDetected)
	}
	if rep.EffortUnknownCount == 0 || len(rep.EffortUnknownModels) != 0 {
		t.Errorf("effort_unknown must be a count without --verbose: %d %v", rep.EffortUnknownCount, rep.EffortUnknownModels)
	}
	f.env.Verbose = true
	if v := buildDoctorReport(f.env); len(v.EffortUnknownModels) != v.EffortUnknownCount {
		t.Errorf("--verbose lists every model: %d of %d", len(v.EffortUnknownModels), v.EffortUnknownCount)
	}
}

// Claude Code on a subscription cannot be listed, but the catalog stand-in is
// the design and the hook still rewrites, so a missing cache is not a warning.
func TestDoctor_ClaudeCodeStandInIsNotAWarning(t *testing.T) {
	f := newDoctorFixture(t)
	writeFile(t, filepath.Join(f.home, ".claude", "settings.json"), claudeSettings(f.bin))

	c := f.check(t, buildDoctorReport(f.env), "claude-code", "session list")
	wantStatus(t, c, doctorOK)
	if !strings.Contains(c.Detail, "catalog stand-in") {
		t.Errorf("detail %q should say the catalog stand-in is used", c.Detail)
	}
}

func TestDoctor_HookMissing(t *testing.T) {
	f := newDoctorFixture(t)
	// PreToolUse only: the observation hooks are missing, and no routing hook
	// for a second harness whose file has no downshift entry at all.
	writeFile(t, filepath.Join(f.home, ".claude", "settings.json"),
		fmt.Sprintf(`{"hooks":{"PreToolUse":[{"matcher":"Task","hooks":[{"type":"command","command":"%s claude-code"}]}]}}`, f.bin))
	writeFile(t, filepath.Join(f.home, ".codex", "hooks.json"), `{"hooks":{"PreToolUse":[{"matcher":"x","hooks":[{"type":"command","command":"/bin/other"}]}]}}`)

	rep := buildDoctorReport(f.env)
	claude := f.check(t, rep, "claude-code", "hooks wired")
	wantStatus(t, claude, doctorWarn) // optional hooks only
	if !strings.Contains(claude.Detail, "missing: PostToolUse") || claude.Fix != "downshift setup claude-code" {
		t.Errorf("claude: %+v", claude)
	}
	codex := f.check(t, rep, "codex", "hooks wired")
	wantStatus(t, codex, doctorFail) // the routing hook is missing
	if rep.Failed == 0 {
		t.Error("a missing routing hook must fail the report")
	}
	var out bytes.Buffer
	rep.writeText(&out, false, false)
	if !strings.Contains(out.String(), "✗ hooks wired") || !strings.Contains(out.String(), "fix: downshift setup codex") {
		t.Errorf("text output:\n%s", out.String())
	}
}

func TestDoctor_StaleBinaryPath(t *testing.T) {
	f := newDoctorFixture(t)
	stale := filepath.Join(f.home, "go", "bin", "downshift")
	writeFile(t, stale, "old")
	writeFile(t, filepath.Join(f.home, ".claude", "settings.json"), claudeSettings(stale))

	rep := buildDoctorReport(f.env)
	c := f.check(t, rep, "claude-code", "hook binary")
	wantStatus(t, c, doctorFail)
	if !strings.Contains(c.Detail, stale) || !strings.Contains(c.Detail, f.bin) {
		t.Errorf("detail must name both binaries: %s", c.Detail)
	}
	want := fmt.Sprintf("sed -i.bak 's#%s #%s #g' %s", stale, f.bin, filepath.Join(f.home, ".claude", "settings.json"))
	if c.Fix != want {
		t.Errorf("fix = %q, want %q", c.Fix, want)
	}

	// A hook binary that no longer exists is just as stale.
	if err := os.Remove(stale); err != nil {
		t.Fatal(err)
	}
	wantStatus(t, f.check(t, buildDoctorReport(f.env), "claude-code", "hook binary"), doctorFail)
}

func TestDoctor_NoEvents(t *testing.T) {
	f := newDoctorFixture(t)
	writeFile(t, filepath.Join(f.home, ".claude", "settings.json"), claudeSettings(f.bin))

	rep := buildDoctorReport(f.env)
	wantStatus(t, f.check(t, rep, "claude-code", "last hook event"), doctorWarn)
	for _, c := range rep.Checks {
		if c.ID == "rewrite honored" {
			t.Errorf("no events, nothing to say about honor: %+v", c)
		}
	}
	if rep.Failed != 0 {
		t.Errorf("no events is a warning, not a failure: %d", rep.Failed)
	}

	// Old events warn too, and name the age.
	f.writeEvents(t, filepath.Join(f.home, ".downshift"), 9*24*time.Hour, false)
	c := f.check(t, buildDoctorReport(f.env), "claude-code", "last hook event")
	wantStatus(t, c, doctorWarn)
	if !strings.Contains(c.Detail, "9d") {
		t.Errorf("detail = %q", c.Detail)
	}
}

func TestDoctor_BothStateDirs(t *testing.T) {
	f := newDoctorFixture(t)
	f.writeEvents(t, filepath.Join(f.home, ".downshift"), time.Hour, true)
	f.writeEvents(t, filepath.Join(f.home, ".harness-downshift"), time.Hour, false)

	rep := buildDoctorReport(f.env)
	c := f.check(t, rep, "", "legacy dir")
	wantStatus(t, c, doctorWarn)
	if !strings.Contains(c.Fix, ".harness-downshift") {
		t.Errorf("fix = %q", c.Fix)
	}

	// Same events in both: ignored, not a warning.
	f.writeEvents(t, filepath.Join(f.home, ".harness-downshift"), time.Hour, true)
	wantStatus(t, f.check(t, buildDoctorReport(f.env), "", "legacy dir"), doctorOK)
}

func TestDoctor_JSONAndExitCode(t *testing.T) {
	f := newDoctorFixture(t)
	t.Setenv("PATH", "") // downshift is not on PATH: a failure
	writeFile(t, filepath.Join(f.home, ".claude", "settings.json"), claudeSettings(f.bin))

	var out, errOut bytes.Buffer
	if rc := runDoctor([]string{"--json"}, &out, &errOut, cmdCat); rc != 1 {
		t.Fatalf("rc = %d, want 1 (downshift not on PATH)\n%s", rc, out.String())
	}
	var rep doctorReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out.String())
	}
	if rep.OK || rep.Failed == 0 || len(rep.Checks) == 0 {
		t.Errorf("report = %+v", rep)
	}

	out.Reset()
	if rc := runDoctor([]string{"extra"}, &out, &errOut, cmdCat); rc != 2 {
		t.Errorf("stray argument: rc = %d, want 2", rc)
	}
}

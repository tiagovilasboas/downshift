// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// Generated wrapper hooks. A setupHook that names a Wrapper is carried by a
// script setup writes under the state directory; the config points at the
// script. No shipped harness uses this yet, so these tests inject a synthetic
// harness into the table, shaped like the one that will (a config the harness
// owns, flat entries in a nested object, a hook that takes no arguments).

const (
	fakeName   = "fakecrew"
	fakeDir    = ".fakecrew"
	fakeConfig = ".fakecrew/config.json"
	fakeStem   = "fakecrew-pre-tool-use"
	fakeArgs   = "fakecrew-hook"
	// guardHook stands for any hook the user already runs in preToolUse.
	guardHook = "/x/guard.py"
	// fakeExisting is a config the way the harness leaves it: the guard hook,
	// and keys setup has no business with.
	fakeExisting = `{
  "jira_hosts": {},
  "agent": {
    "kiro_hooks": {
      "preToolUse": [
        {
          "matcher": "*",
          "command": "` + guardHook + `"
        }
      ]
    }
  },
  "timezone": "America/Sao_Paulo"
}
`
)

// withFakeHarness adds the synthetic harness to the table for one test.
func withFakeHarness(t *testing.T) {
	t.Helper()
	saved := setupTargets
	setupTargets = append(append([]setupTarget{}, saved...), setupTarget{
		Name:        fakeName,
		ConfigDir:   fakeDir,
		File:        fakeConfig,
		HooksPath:   []string{"agent", "kiro_hooks"},
		RequireFile: true,
		Hooks:       []setupHook{{Event: "preToolUse", Matcher: "*", Args: fakeArgs, Wrapper: fakeStem}},
		Notes:       []string{"Restart the harness."},
	})
	t.Cleanup(func() { setupTargets = saved })
}

// wrapperFor is where setup keeps the fake harness wrapper in env e.
func (e *setupEnv) wrapperFor() string {
	return e.path(filepath.Join(".downshift", "bin", wrapperFile(fakeStem, runtime.GOOS)))
}

// preToolUseCommands lists the commands under agent.kiro_hooks.preToolUse.
func preToolUseCommands(t *testing.T, config string) []string {
	t.Helper()
	doc := parseDoc(t, config)
	agent, _ := doc["agent"].(map[string]any)
	hooks, _ := agent["kiro_hooks"].(map[string]any)
	list, _ := hooks["preToolUse"].([]any)
	var out []string
	for _, item := range list {
		out = append(out, item.(map[string]any)["command"].(string))
	}
	return out
}

func TestSetupGeneratedWrapperWiresTheHookAndKeepsTheRest(t *testing.T) {
	withFakeHarness(t)
	e := newSetupEnv(t, fakeDir)
	e.write(fakeConfig, fakeExisting)

	code, out, errOut := e.run(fakeName, "--yes")
	if code != 0 {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	got := preToolUseCommands(t, e.read(fakeConfig))
	want := []string{guardHook, e.wrapperFor()}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("preToolUse = %v, want the guard kept and the wrapper added: %v", got, want)
	}
	doc := parseDoc(t, e.read(fakeConfig))
	if doc["timezone"] != "America/Sao_Paulo" {
		t.Errorf("an unrelated key changed: %v", doc["timezone"])
	}
	if _, ok := doc["jira_hosts"]; !ok {
		t.Error("an unrelated key was dropped")
	}
	// Key order is the user's: jira_hosts stays first.
	if raw := e.read(fakeConfig); strings.Index(raw, "jira_hosts") > strings.Index(raw, `"agent"`) {
		t.Errorf("key order changed:\n%s", raw)
	}
	if !strings.Contains(out, "Restart the harness.") {
		t.Errorf("no restart note:\n%s", out)
	}
}

func TestSetupGeneratedWrapperWrapperRunsTheRunningBinary(t *testing.T) {
	withFakeHarness(t)
	e := newSetupEnv(t, fakeDir)
	e.write(fakeConfig, fakeExisting)
	if code, out, _ := e.run(fakeName, "--yes"); code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	body := e.read(filepath.Join(".downshift", "bin", wrapperFile(fakeStem, runtime.GOOS)))
	if !strings.Contains(body, testBin) || !strings.Contains(body, fakeArgs) {
		t.Errorf("wrapper does not run %s %s:\n%s", testBin, fakeArgs, body)
	}
	if runtime.GOOS == "windows" {
		return
	}
	st, err := os.Stat(e.wrapperFor())
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o700 {
		t.Errorf("wrapper mode = %v, want 0700: the harness must be able to run it and nobody else", st.Mode().Perm())
	}
}

func TestSetupGeneratedWrapperIsIdempotent(t *testing.T) {
	withFakeHarness(t)
	e := newSetupEnv(t, fakeDir)
	e.write(fakeConfig, fakeExisting)
	if code, out, _ := e.run(fakeName, "--yes"); code != 0 {
		t.Fatalf("first run exit %d\n%s", code, out)
	}
	before := e.read(fakeConfig)
	code, out, _ := e.run(fakeName, "--yes")
	if code != 0 {
		t.Fatalf("second run exit %d\n%s", code, out)
	}
	if after := e.read(fakeConfig); after != before {
		t.Errorf("a second run changed the config:\n%s", after)
	}
	if n := len(preToolUseCommands(t, e.read(fakeConfig))); n != 2 {
		t.Errorf("preToolUse has %d entries after two runs, want 2", n)
	}
	if !strings.Contains(out, "already up to date") {
		t.Errorf("second run should report up to date:\n%s", out)
	}
}

func TestSetupGeneratedWrapperDryRunWritesNothing(t *testing.T) {
	withFakeHarness(t)
	e := newSetupEnv(t, fakeDir)
	e.write(fakeConfig, fakeExisting)
	code, out, _ := e.run(fakeName, "--dry-run")
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	if got := e.read(fakeConfig); got != fakeExisting {
		t.Errorf("dry run changed the config:\n%s", got)
	}
	if _, err := os.Stat(e.wrapperFor()); !os.IsNotExist(err) {
		t.Errorf("dry run wrote the wrapper (stat err = %v)", err)
	}
	// The diff names both files, so the operator sees what would run.
	if !strings.Contains(out, fakeStem) || !strings.Contains(out, "config.json") {
		t.Errorf("dry run does not show the wrapper and the config:\n%s", out)
	}
}

// setup never creates the harness's config: a missing file means the harness is
// not set up, and writing one here would be a guess.
func TestSetupGeneratedWrapperWithoutAConfigWritesNeitherConfigNorWrapper(t *testing.T) {
	withFakeHarness(t)
	e := newSetupEnv(t, fakeDir)
	code, out, _ := e.run(fakeName, "--yes")
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	if _, err := os.Stat(e.path(fakeConfig)); !os.IsNotExist(err) {
		t.Errorf("setup created the harness config (stat err = %v)", err)
	}
	if _, err := os.Stat(e.wrapperFor()); !os.IsNotExist(err) {
		t.Errorf("setup wrote a wrapper nothing points at (stat err = %v)", err)
	}
	if !strings.Contains(out, "skipped") || !strings.Contains(out, "not set up yet") {
		t.Errorf("no explanation for the skip:\n%s", out)
	}
}

// A wrapper the operator registered by hand (the install guide's way) is theirs.
func TestSetupGeneratedWrapperLeavesAHandRegisteredWrapperAlone(t *testing.T) {
	withFakeHarness(t)
	const checkout = "/home/dev/downshift/scripts/fakecrew-pre-tool-use.sh"
	manual := strings.Replace(fakeExisting, guardHook, checkout, 1)
	e := newSetupEnv(t, fakeDir)
	e.write(fakeConfig, manual)

	code, out, _ := e.run(fakeName, "--yes")
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	if got := e.read(fakeConfig); got != manual {
		t.Errorf("a hand-registered wrapper was rewritten:\n%s", got)
	}
	if _, err := os.Stat(e.wrapperFor()); !os.IsNotExist(err) {
		t.Errorf("a second wrapper was generated beside the manual one (stat err = %v)", err)
	}
	if !strings.Contains(out, "already wired through "+checkout) {
		t.Errorf("no note about the existing hook:\n%s", out)
	}
}

// A moved binary changes the wrapper, not the config that points at it.
func TestSetupGeneratedWrapperRewritesTheWrapperWhenTheBinaryMoves(t *testing.T) {
	withFakeHarness(t)
	e := newSetupEnv(t, fakeDir)
	e.write(fakeConfig, fakeExisting)
	if code, out, _ := e.run(fakeName, "--yes"); code != 0 {
		t.Fatalf("first run exit %d\n%s", code, out)
	}
	configBefore := e.read(fakeConfig)

	const moved = "/usr/local/bin/downshift"
	if code, out, _ := e.runBin(moved, fakeName, "--yes"); code != 0 {
		t.Fatalf("second run exit %d\n%s", code, out)
	}
	body, err := os.ReadFile(e.wrapperFor())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), moved) || strings.Contains(string(body), testBin) {
		t.Errorf("wrapper still runs the old binary:\n%s", body)
	}
	if got := e.read(fakeConfig); got != configBefore {
		t.Errorf("moving the binary changed the config:\n%s", got)
	}
	if matches, _ := filepath.Glob(e.wrapperFor() + setupBackupInfix + "*"); len(matches) == 0 {
		t.Error("the old wrapper was not backed up")
	}
}

func TestSetupGeneratedWrapperUndoRestoresTheConfigAndRemovesTheWrapper(t *testing.T) {
	withFakeHarness(t)
	e := newSetupEnv(t, fakeDir)
	e.write(fakeConfig, fakeExisting)
	if code, out, _ := e.run(fakeName, "--yes"); code != 0 {
		t.Fatalf("setup exit %d\n%s", code, out)
	}
	if code, out, _ := e.run(fakeName, "--undo", "--yes"); code != 0 {
		t.Fatalf("undo exit %d\n%s", code, out)
	}
	if got := e.read(fakeConfig); got != fakeExisting {
		t.Errorf("undo did not restore the config:\n%s", got)
	}
	if _, err := os.Stat(e.wrapperFor()); !os.IsNotExist(err) {
		t.Errorf("undo left the wrapper behind (stat err = %v)", err)
	}
}

// The config must never point at a wrapper that was not written. A wrapper
// that cannot even be planned stops the whole target before anything is written.
func TestSetupGeneratedWrapperDoesNotWriteTheConfigWhenTheWrapperCannotBePlanned(t *testing.T) {
	withFakeHarness(t)
	e := newSetupEnv(t, fakeDir)
	e.write(fakeConfig, fakeExisting)
	// A file where the wrapper directory should be: the wrapper cannot be read.
	e.write(filepath.Join(".downshift", "bin"), "not a directory")

	code, out, errOut := e.run(fakeName, "--yes")
	if code == 0 {
		t.Fatalf("a wrapper that cannot be planned should not exit 0\n%s\n%s", out, errOut)
	}
	if got := e.read(fakeConfig); got != fakeExisting {
		t.Errorf("the config was written although its wrapper was not:\n%s", got)
	}
	if !strings.Contains(errOut, "nothing was written for fakecrew") {
		t.Errorf("no explanation:\n%s", errOut)
	}
}

// A wrapper that plans fine but fails when written must also keep the config
// from being written (the write-time dependency, not the planning one).
func TestSetupGeneratedWrapperDoesNotWriteTheConfigWhenTheWrapperWriteFails(t *testing.T) {
	withFakeHarness(t)
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs a directory the current user cannot write to")
	}
	e := newSetupEnv(t, fakeDir)
	e.write(fakeConfig, fakeExisting)
	bin := e.path(filepath.Join(".downshift", "bin"))
	if err := os.MkdirAll(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(bin, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(bin, 0o700) })

	code, out, errOut := e.run(fakeName, "--yes")
	if code == 0 {
		t.Fatalf("a failed wrapper write should not exit 0\n%s\n%s", out, errOut)
	}
	if got := e.read(fakeConfig); got != fakeExisting {
		t.Errorf("the config was written although its wrapper was not:\n%s", got)
	}
	if !strings.Contains(errOut, "not written") || !strings.Contains(errOut, "the config would point at it") {
		t.Errorf("no explanation for the skipped config:\n%s", errOut)
	}
}

func TestSetupGeneratedWrapperRefusesABinaryPathItCannotQuote(t *testing.T) {
	withFakeHarness(t)
	e := newSetupEnv(t, fakeDir)
	e.write(fakeConfig, fakeExisting)
	code, _, errOut := e.runBin("/opt/ds/down\nshift", fakeName, "--yes")
	if code == 0 {
		t.Fatal("a binary path with a newline should fail")
	}
	if got := e.read(fakeConfig); got != fakeExisting {
		t.Errorf("the config changed:\n%s", got)
	}
	if !strings.Contains(errOut, "control character") {
		t.Errorf("no reason given:\n%s", errOut)
	}
}

func TestSetupGeneratedWrapperCreatesTheHookObjectsWhenTheConfigHasNone(t *testing.T) {
	withFakeHarness(t)
	e := newSetupEnv(t, fakeDir)
	e.write(fakeConfig, `{"timezone": "UTC"}`)
	if code, out, _ := e.run(fakeName, "--yes"); code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	got := preToolUseCommands(t, e.read(fakeConfig))
	if len(got) != 1 || got[0] != e.wrapperFor() {
		t.Errorf("preToolUse = %v, want only the wrapper", got)
	}
}

func TestSetupGeneratedWrapperInvalidConfigIsLeftUntouched(t *testing.T) {
	withFakeHarness(t)
	e := newSetupEnv(t, fakeDir)
	e.write(fakeConfig, `{"agent": `)
	code, _, errOut := e.run(fakeName, "--yes")
	if code == 0 || !strings.Contains(errOut, "not valid JSON") {
		t.Fatalf("exit %d, err %q", code, errOut)
	}
	if got := e.read(fakeConfig); got != `{"agent": ` {
		t.Errorf("an invalid config was rewritten: %q", got)
	}
	if _, err := os.Stat(e.wrapperFor()); !os.IsNotExist(err) {
		t.Errorf("a wrapper was written for a config setup could not edit (stat err = %v)", err)
	}
}

// ---- the wrapper itself --------------------------------------------------

func TestWrapperScriptPOSIX(t *testing.T) {
	s, err := wrapperScript("linux", "/opt/my tools/it's/downshift", "fakecrew-hook")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(s, "#!/bin/sh\n") {
		t.Errorf("no shebang:\n%s", s)
	}
	// Spaces and a single quote in the path are quoted, not interpreted.
	if !strings.Contains(s, `bin='/opt/my tools/it'\''s/downshift'`) {
		t.Errorf("path is not shell-quoted:\n%s", s)
	}
	if !strings.Contains(s, `[ -x "$bin" ] || exit 0`) {
		t.Errorf("a missing binary must let the call through:\n%s", s)
	}
	if !strings.HasSuffix(s, `exec "$bin" fakecrew-hook`+"\n") {
		t.Errorf("exec must be last so stdin and the exit code pass through:\n%s", s)
	}
	if strings.Contains(s, "\r") {
		t.Errorf("POSIX script has CRs:\n%q", s)
	}
}

func TestWrapperScriptWindows(t *testing.T) {
	s, err := wrapperScript("windows", `C:\Program Files\ds\downshift.exe`, "fakecrew-hook")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(s, "@echo off\r\n") || !strings.Contains(s, "\r\n") || strings.Contains(strings.ReplaceAll(s, "\r\n", ""), "\n") {
		t.Errorf("a .cmd needs CRLF line endings throughout:\n%q", s)
	}
	for _, want := range []string{
		`if not exist "C:\Program Files\ds\downshift.exe" exit /b 0`,
		`"C:\Program Files\ds\downshift.exe" fakecrew-hook`,
		"exit /b %ERRORLEVEL%",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q:\n%s", want, s)
		}
	}
	// A percent sign in the path would be expanded by cmd.
	s2, err := wrapperScript("windows", `C:\100%\downshift.exe`, "fakecrew-hook")
	if err != nil || !strings.Contains(s2, `C:\100%%\downshift.exe`) {
		t.Errorf("percent not escaped: %v\n%s", err, s2)
	}
}

func TestWrapperScriptRefusesWhatItCannotQuote(t *testing.T) {
	for _, tc := range []struct{ goos, bin string }{
		{"linux", "/a\nb"}, {"linux", "/a\rb"}, {"linux", "/a\x00b"},
		{"windows", "C:\\a\nb"}, {"windows", `C:\a"b\downshift.exe`},
	} {
		if _, err := wrapperScript(tc.goos, tc.bin, "fakecrew-hook"); err == nil {
			t.Errorf("%s %q: want an error", tc.goos, tc.bin)
		}
	}
}

func TestWrapperFileFollowsTheOS(t *testing.T) {
	if got := wrapperFile(fakeStem, "windows"); got != fakeStem+".cmd" {
		t.Errorf("windows: %s", got)
	}
	for _, goos := range []string{"linux", "darwin"} {
		if got := wrapperFile(fakeStem, goos); got != fakeStem+".sh" {
			t.Errorf("%s: %s", goos, got)
		}
	}
}

func TestIsWrapperCommand(t *testing.T) {
	for _, c := range []string{
		"/a/scripts/fakecrew-pre-tool-use.sh",
		"/home/me/.downshift/bin/fakecrew-pre-tool-use.sh",
		`C:\ds\bin\fakecrew-pre-tool-use.cmd`,
		"  /a/fakecrew-pre-tool-use.sh  ",
	} {
		if !isWrapperCommand(c, fakeStem) {
			t.Errorf("%q not recognised", c)
		}
	}
	for _, c := range []string{
		"", guardHook, "/a/fakecrew-pre-tool-use", "/a/fakecrew-pre-tool-use.py",
		"/a/not-fakecrew-pre-tool-use.sh", "/a/fakecrew-pre-tool-use.sh.bak",
		"/opt/ds/downshift fakecrew-hook",
	} {
		if isWrapperCommand(c, fakeStem) {
			t.Errorf("%q wrongly recognised", c)
		}
	}
}

// The generated wrapper is what the harness runs, so run it: the hook payload on
// stdin must reach `<binary> fakecrew-hook`, and its exit code (2 blocks a spawn)
// must come back unchanged.
func TestGeneratedWrapperPassesStdinAndExitCodeThrough(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell wrapper")
	}
	dir := t.TempDir()
	fake := filepath.Join(dir, "fake downshift") // a space: the quoting is exercised too
	script := "#!/bin/sh\n" +
		`[ "$1" = fakecrew-hook ] || exit 64` + "\n" +
		`cat > "$0.stdin"` + "\n" +
		`exit "${FAKE_EXIT:-0}"` + "\n"
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	body, err := wrapperScript("linux", fake, fakeArgs)
	if err != nil {
		t.Fatal(err)
	}
	wrapper := filepath.Join(dir, "wrapper.sh")
	if err := os.WriteFile(wrapper, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, code := range []string{"0", "2"} {
		cmd := exec.Command(wrapper)
		cmd.Env = append(os.Environ(), "FAKE_EXIT="+code)
		cmd.Stdin = strings.NewReader(`{"tool_name":"spawn_run"}`)
		err := cmd.Run()
		got := 0
		if ee, ok := err.(*exec.ExitError); ok {
			got = ee.ExitCode()
		} else if err != nil {
			t.Fatalf("exit %s: %v", code, err)
		}
		if strconv.Itoa(got) != code {
			t.Errorf("wrapper exit = %d, want %s", got, code)
		}
		seen, _ := os.ReadFile(fake + ".stdin")
		if string(seen) != `{"tool_name":"spawn_run"}` {
			t.Errorf("the payload did not reach the binary: %q", seen)
		}
	}
	// A binary that is gone lets the call through instead of failing the hook.
	if err := os.Remove(fake); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(wrapper)
	cmd.Stdin = strings.NewReader("{}")
	if err := cmd.Run(); err != nil {
		t.Errorf("a missing binary must exit 0, got %v", err)
	}
}

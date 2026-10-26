// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// Generated wrapper hooks. A setupHook that names a Wrapper is carried by a
// script setup writes under the state directory; the config points at it. No
// shipped harness uses this yet, so these tests add a synthetic one to the
// table, shaped like the one that will: a config the harness owns and a hook
// that takes no arguments.

const (
	fakeName   = "fakecrew"
	fakeDir    = ".fakecrew"
	fakeConfig = ".fakecrew/config.json"
	fakeStem   = "fakecrew-pre-tool-use"
	fakeArgs   = "fakecrew-hook"
	guardHook  = "/x/guard.py" // a hook the operator already runs
	fakeBefore = `{
  "timezone": "America/Sao_Paulo",
  "agent": {"kiro_hooks": {"preToolUse": [{"matcher": "*", "command": "` + guardHook + `"}]}}
}
`
)

func withFakeHarness(t *testing.T) {
	t.Helper()
	saved := setupTargets
	setupTargets = append(append([]setupTarget{}, saved...), setupTarget{
		Name: fakeName, ConfigDir: fakeDir, File: fakeConfig, RequireFile: true,
		HooksPath: []string{"agent", "kiro_hooks"},
		Hooks:     []setupHook{{Event: "preToolUse", Matcher: "*", Args: fakeArgs, Wrapper: fakeStem}},
	})
	t.Cleanup(func() { setupTargets = saved })
}

func (e *setupEnv) wrapperRel() string {
	return filepath.Join(".downshift", "bin", wrapperFile(fakeStem, runtime.GOOS))
}

func preToolUseCommands(t *testing.T, config string) []string {
	t.Helper()
	agent, _ := parseDoc(t, config)["agent"].(map[string]any)
	hooks, _ := agent["kiro_hooks"].(map[string]any)
	list, _ := hooks["preToolUse"].([]any)
	var out []string
	for _, item := range list {
		out = append(out, item.(map[string]any)["command"].(string))
	}
	return out
}

func newFakeEnv(t *testing.T, config string) *setupEnv {
	t.Helper()
	withFakeHarness(t)
	e := newSetupEnv(t, fakeDir)
	if config != "" {
		e.write(fakeConfig, config)
	}
	return e
}

func TestSetupGeneratedWrapperIsRegisteredNextToTheExistingHook(t *testing.T) {
	e := newFakeEnv(t, fakeBefore)
	if code, out, errOut := e.run(fakeName, "--yes"); code != 0 {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	got := strings.Join(preToolUseCommands(t, e.read(fakeConfig)), "|")
	if want := guardHook + "|" + e.path(e.wrapperRel()); got != want {
		t.Fatalf("preToolUse = %s, want %s", got, want)
	}
	if body := e.read(e.wrapperRel()); !strings.Contains(body, testBin) || !strings.Contains(body, fakeArgs) {
		t.Errorf("the wrapper does not run %s %s:\n%s", testBin, fakeArgs, body)
	}
	if parseDoc(t, e.read(fakeConfig))["timezone"] != "America/Sao_Paulo" {
		t.Error("an unrelated key changed")
	}
	if runtime.GOOS != "windows" {
		if st, err := os.Stat(e.path(e.wrapperRel())); err != nil || st.Mode().Perm() != 0o700 {
			t.Errorf("wrapper mode = %v (%v), want 0700", st.Mode().Perm(), err)
		}
	}
	// Idempotent: a second run changes nothing.
	before := e.read(fakeConfig)
	if code, out, _ := e.run(fakeName, "--yes"); code != 0 || e.read(fakeConfig) != before || !strings.Contains(out, "already up to date") {
		t.Errorf("second run exit %d, changed=%v\n%s", code, e.read(fakeConfig) != before, out)
	}
}

// setup never creates a config the harness owns, nor a wrapper nothing points at.
func TestSetupGeneratedWrapperWithoutAConfigWritesNothing(t *testing.T) {
	e := newFakeEnv(t, "")
	code, out, _ := e.run(fakeName, "--yes")
	if code != 0 || !strings.Contains(out, "not set up yet") {
		t.Fatalf("exit %d\n%s", code, out)
	}
	for _, rel := range []string{fakeConfig, e.wrapperRel()} {
		if _, err := os.Stat(e.path(rel)); !os.IsNotExist(err) {
			t.Errorf("%s was written (stat err = %v)", rel, err)
		}
	}
}

// A wrapper registered by hand from a checkout is the same hook: it is pointed
// at the generated one, like a stale binary path is for the other harnesses.
func TestSetupGeneratedWrapperReplacesAHandRegisteredOne(t *testing.T) {
	manual := strings.Replace(fakeBefore, guardHook, "/home/dev/ds/scripts/"+fakeStem+".sh", 1)
	e := newFakeEnv(t, manual)
	if code, out, _ := e.run(fakeName, "--yes"); code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	if got := preToolUseCommands(t, e.read(fakeConfig)); len(got) != 1 || got[0] != e.path(e.wrapperRel()) {
		t.Errorf("preToolUse = %v, want only the generated wrapper", got)
	}
}

func TestSetupGeneratedWrapperUndo(t *testing.T) {
	t.Run("removes a wrapper setup created and restores the config", func(t *testing.T) {
		e := newFakeEnv(t, fakeBefore)
		e.run(fakeName, "--yes")
		if code, out, _ := e.run(fakeName, "--undo", "--yes"); code != 0 {
			t.Fatalf("undo exit %d\n%s", code, out)
		}
		if e.read(fakeConfig) != fakeBefore {
			t.Error("the config was not restored")
		}
		if _, err := os.Stat(e.path(e.wrapperRel())); !os.IsNotExist(err) {
			t.Errorf("the wrapper is still there (stat err = %v)", err)
		}
	})
	t.Run("restores a wrapper that was there before", func(t *testing.T) {
		e := newFakeEnv(t, fakeBefore)
		const previous = "#!/bin/sh\n# an older wrapper\nexit 0\n"
		e.write(e.wrapperRel(), previous)
		e.run(fakeName, "--yes")
		if e.read(e.wrapperRel()) == previous {
			t.Fatal("setup did not replace the old wrapper; the test proves nothing")
		}
		if code, out, _ := e.run(fakeName, "--undo", "--yes"); code != 0 {
			t.Fatalf("undo exit %d\n%s", code, out)
		}
		if got := e.read(e.wrapperRel()); got != previous {
			t.Errorf("the earlier wrapper was not restored:\n%s", got)
		}
	})
}

// The config must never point at a wrapper that was not written.
func TestSetupGeneratedWrapperFailureKeepsTheConfig(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs a directory the current user cannot write to")
	}
	e := newFakeEnv(t, fakeBefore)
	state := e.path(".downshift") // exists, but bin/ cannot be created inside it
	if err := os.MkdirAll(state, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(state, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(state, 0o700) })

	code, _, errOut := e.run(fakeName, "--yes")
	if code == 0 || e.read(fakeConfig) != fakeBefore || !strings.Contains(errOut, "the config would point at it") {
		t.Errorf("exit %d, config kept=%v\n%s", code, e.read(fakeConfig) == fakeBefore, errOut)
	}
}

// The wrapper runs on every tool call: bin/ is 0700 even if it was left wider,
// and the harness's own directory is left as it was.
func TestSetupGeneratedWrapperClosesAWideBinDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits")
	}
	e := newFakeEnv(t, fakeBefore)
	bin := e.path(filepath.Join(".downshift", "bin"))
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if code, out, _ := e.run(fakeName, "--yes"); code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	if st, _ := os.Stat(bin); st.Mode().Perm() != 0o700 {
		t.Errorf("bin/ mode = %v, want 0700", st.Mode().Perm())
	}
	if st, _ := os.Stat(e.path(fakeDir)); st.Mode().Perm() != 0o755 {
		t.Errorf("the harness directory was changed to %v", st.Mode().Perm())
	}
}

// ---- the script ----------------------------------------------------------

func TestWrapperScriptPOSIX(t *testing.T) {
	s, err := wrapperScript("linux", "/opt/my tools/it's/downshift", fakeArgs)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"#!/bin/sh\n",
		`recorded='/opt/my tools/it'\''s/downshift'`, // quoted, not interpreted
		`if [ ! -x "$bin" ]; then bin=$(command -v downshift 2>/dev/null); fi`,
		`echo "downshift: $recorded not found and no downshift on PATH, hook skipped" >&2; exit 0; }`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q:\n%s", want, s)
		}
	}
	if !strings.HasSuffix(s, `exec "$bin" `+fakeArgs+"\n") || strings.Contains(s, "\r") {
		t.Errorf("exec must be last and the script LF-only:\n%q", s)
	}
}

func TestWrapperScriptWindows(t *testing.T) {
	s, err := wrapperScript("windows", `C:\Program Files\ds\downshift.exe`, fakeArgs)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(s, "@echo off\r\n") || strings.Contains(strings.ReplaceAll(s, "\r\n", ""), "\n") {
		t.Errorf("a .cmd needs CRLF throughout:\n%q", s)
	}
	for _, want := range []string{
		`if not exist "C:\Program Files\ds\downshift.exe" (`,
		"echo downshift: binary not found, hook skipped 1>&2",
		`"C:\Program Files\ds\downshift.exe" ` + fakeArgs,
		"exit /b %ERRORLEVEL%",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q:\n%s", want, s)
		}
	}
	if s2, err := wrapperScript("windows", `C:\100%\downshift.exe`, fakeArgs); err != nil || !strings.Contains(s2, `C:\100%%\downshift.exe`) {
		t.Errorf("a percent sign must be doubled: %v\n%s", err, s2)
	}
}

func TestWrapperScriptRefusesWhatItCannotQuote(t *testing.T) {
	for _, tc := range []struct{ goos, bin string }{
		{"linux", "/a\nb"}, {"linux", "/a\rb"}, {"linux", "/a\x00b"},
		{"windows", "C:\\a\nb"}, {"windows", `C:\a"b\downshift.exe`},
	} {
		if _, err := wrapperScript(tc.goos, tc.bin, fakeArgs); err == nil {
			t.Errorf("%s %q: want an error", tc.goos, tc.bin)
		}
	}
}

func TestWrapperNaming(t *testing.T) {
	if wrapperFile(fakeStem, "windows") != fakeStem+".cmd" || wrapperFile(fakeStem, "linux") != fakeStem+".sh" {
		t.Error("wrong extension")
	}
	for _, c := range []string{"/a/" + fakeStem + ".sh", `C:\Program Files\ds\` + fakeStem + ".cmd"} {
		if !isWrapperCommand(c, fakeStem) {
			t.Errorf("%q not recognised", c)
		}
	}
	for _, c := range []string{"", guardHook, "/a/" + fakeStem, "/a/not-" + fakeStem + ".sh", "/a/" + fakeStem + ".sh.bak", "/opt/ds/downshift " + fakeArgs} {
		if isWrapperCommand(c, fakeStem) {
			t.Errorf("%q wrongly recognised", c)
		}
	}
}

// The generated script is what the harness runs, so run it: stdin and the exit
// code (2 blocks a spawn) pass through, a hostile binary path stays data, and a
// vanished binary lets the call through with a line on stderr.
func TestGeneratedWrapperRuns(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell wrapper")
	}
	dir := t.TempDir()
	hostile := filepath.Join(dir, "a'b$(touch PWNED1)`touch PWNED2`;touch PWNED3 c")
	fakeBinary(t, hostile)
	body, err := wrapperScript("linux", hostile, fakeArgs)
	if err != nil {
		t.Fatal(err)
	}
	wrapper := filepath.Join(dir, "wrapper.sh")
	if err := os.WriteFile(wrapper, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	run := func(exit string) (int, string) { return runWrapper(t, wrapper, os.Getenv("PATH"), exit) }
	for _, want := range []int{0, 2} {
		if got, _ := run(strconv.Itoa(want)); got != want {
			t.Errorf("exit = %d, want %d", got, want)
		}
	}
	if seen, _ := os.ReadFile(hostile + ".stdin"); string(seen) != `{"tool_name":"spawn_run"}` {
		t.Errorf("the payload did not reach the binary: %q", seen)
	}
	for _, p := range []string{"PWNED1", "PWNED2", "PWNED3"} {
		if _, err := os.Stat(filepath.Join(dir, p)); err == nil {
			t.Errorf("INJECTION: %s was created by the binary path", p)
		}
	}
	if err := os.Remove(hostile); err != nil {
		t.Fatal(err)
	}
	empty := t.TempDir() // no downshift on PATH either
	if code, stderr := runWrapper(t, wrapper, empty, "0"); code != 0 || !strings.Contains(stderr, "no downshift on PATH, hook skipped") {
		t.Errorf("a vanished binary must exit 0 and say so: exit %d, stderr %q", code, stderr)
	}
}

// When the recorded binary has moved, the POSIX wrapper runs the downshift on
// PATH with the payload and passes its exit code through.
func TestGeneratedWrapperFallsBackToPATH(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell wrapper")
	}
	dir := t.TempDir()
	body, err := wrapperScript("linux", filepath.Join(dir, "moved", "downshift"), fakeArgs)
	if err != nil {
		t.Fatal(err)
	}
	wrapper := filepath.Join(dir, "wrapper.sh")
	if err := os.WriteFile(wrapper, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "path-bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	onPATH := filepath.Join(bin, "downshift")
	fakeBinary(t, onPATH)
	pathEnv := bin + string(os.PathListSeparator) + "/bin:/usr/bin" // cat lives in /bin
	for _, want := range []int{0, 2} {
		if got, stderr := runWrapper(t, wrapper, pathEnv, strconv.Itoa(want)); got != want {
			t.Errorf("exit = %d, want %d (stderr %q)", got, want, stderr)
		}
	}
	if seen, _ := os.ReadFile(onPATH + ".stdin"); string(seen) != `{"tool_name":"spawn_run"}` {
		t.Errorf("the payload did not reach the PATH downshift: %q", seen)
	}
}

// fakeBinary writes a stand-in binary at path. It accepts only fakeArgs, copies
// its stdin next to itself and exits with $FAKE_EXIT.
func fakeBinary(t *testing.T, path string) {
	t.Helper()
	script := "#!/bin/sh\n" + `[ "$1" = ` + fakeArgs + ` ] || exit 64` + "\n" +
		`cat > "$0.stdin"` + "\n" + `exit "${FAKE_EXIT:-0}"` + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

// runWrapper runs the wrapper with the hook payload on stdin, FAKE_EXIT set to
// exit and PATH set to pathEnv. It returns the exit code and stderr.
func runWrapper(t *testing.T, wrapper, pathEnv, exit string) (int, string) {
	t.Helper()
	cmd := exec.Command(wrapper)
	cmd.Dir = filepath.Dir(wrapper)
	cmd.Env = append(os.Environ(), "PATH="+pathEnv, "FAKE_EXIT="+exit)
	cmd.Stdin = strings.NewReader(`{"tool_name":"spawn_run"}`)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	code := 0
	if err := cmd.Run(); err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatal(err)
		}
		code = ee.ExitCode()
	}
	return code, stderr.String()
}

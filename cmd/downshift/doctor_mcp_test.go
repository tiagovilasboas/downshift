// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/paths"
	"github.com/tiagovilasboas/downshift-ai/internal/setup"
	"github.com/tiagovilasboas/downshift-ai/internal/testenv"
)

// mcpTestEnv is a temp HOME with a .claude.json and a fake running binary, so
// the MCP check can be driven without the real claude CLI or a real server.
// HOME and the state dir are isolated as the repo's doctor tests do.
func mcpTestEnv(t *testing.T) (home string, exe string) {
	t.Helper()
	home = t.TempDir()
	// The doctor reports symlink-resolved paths. macOS puts TempDir under
	// /var, a symlink to /private/var, so build the fixtures on the real path.
	if real, err := filepath.EvalSymlinks(home); err == nil {
		home = real
	}
	testenv.SetHome(t, home)
	t.Setenv(paths.EnvStateDir, filepath.Join(home, "state"))
	exe = filepath.Join(home, "bin", "downshift")
	writeFile(t, exe, "#!/bin/sh\n")
	if err := os.Chmod(exe, 0o755); err != nil {
		t.Fatal(err)
	}
	return home, exe
}

// writeClaudeEntry writes a .claude.json whose mcpServers.downshift runs
// command. managed adds the setup env marker, as the real entry carries it.
func writeClaudeEntry(t *testing.T, home, command string, managed bool) {
	t.Helper()
	env := ""
	if managed {
		env = `,"env":{"` + setup.McpMarkerKey + `":"` + setup.McpMarkerValue + `"}`
	}
	doc := `{"mcpServers":{"downshift":{"type":"stdio","command":"` + command + `","args":["mcp","serve"]` + env + `}}}`
	writeFile(t, filepath.Join(home, ".claude.json"), doc)
}

// collectMcp runs the MCP check with an injected env and returns the rows by id.
type mcpRow struct{ status, detail, fix string }

func collectMcp(env mcpDoctorEnv) map[string]mcpRow {
	got := map[string]mcpRow{}
	checkClaudeCodeMcpWith(func(id, status, detail, fix string) {
		got[id] = mcpRow{status, detail, fix}
	}, env)
	return got
}

// envFor builds an mcpDoctorEnv with a fixed running binary and probe.
func envFor(home, exe string, probe func(context.Context, string) (mcpProbeResult, error)) mcpDoctorEnv {
	return mcpDoctorEnv{
		Home:       home,
		Executable: func() (string, error) { return exe, nil },
		Probe:      probe,
	}
}

func TestCheckClaudeCodeMcpSkipsOtherHarnesses(t *testing.T) {
	for _, h := range []string{"kirocrew", "cursor", "codex", ""} {
		checkClaudeCodeMcp(func(id, _, _, _ string) {
			t.Errorf("harness %q reported MCP row %s", h, id)
		}, h)
	}
}

func TestCheckClaudeCodeMcpNoEntry(t *testing.T) {
	home, exe := mcpTestEnv(t)
	// No .claude.json at all: the reader returns not found, not an error.
	got := collectMcp(envFor(home, exe, func(context.Context, string) (mcpProbeResult, error) {
		t.Fatal("probe must not run without a registered entry")
		return mcpProbeResult{}, nil
	}))
	reg, ok := got["mcp registered"]
	if !ok || reg.status != doctorWarn || !strings.Contains(reg.detail, "no mcpServers.downshift") {
		t.Fatalf("registered row = %+v", reg)
	}
	if reg.fix != mcpSetupFix {
		t.Errorf("fix = %q, want %q", reg.fix, mcpSetupFix)
	}
	if _, ok := got["mcp command"]; ok {
		t.Error("command row should not appear without an entry")
	}
	if _, ok := got["mcp answers"]; ok {
		t.Error("answers row should not appear without an entry")
	}
}

func TestCheckClaudeCodeMcpMismatchedCommand(t *testing.T) {
	home, exe := mcpTestEnv(t)
	// The entry points at a different real binary than the one running.
	other := filepath.Join(home, "bin", "other-downshift")
	writeFile(t, other, "#!/bin/sh\n")
	if err := os.Chmod(other, 0o755); err != nil {
		t.Fatal(err)
	}
	writeClaudeEntry(t, home, other, true)

	probed := false
	got := collectMcp(envFor(home, exe, func(context.Context, string) (mcpProbeResult, error) {
		probed = true
		return mcpProbeResult{Tools: 7}, nil
	}))
	if r := got["mcp registered"]; r.status != doctorOK {
		t.Errorf("registered row = %+v, want ok", r)
	}
	// The row names the resolved binary: on macOS the temp dir under /var
	// resolves to /private/var.
	exeReal, err := filepath.EvalSymlinks(exe)
	if err != nil {
		t.Fatal(err)
	}
	cmd, ok := got["mcp command"]
	if !ok || cmd.status != doctorWarn || !strings.Contains(cmd.detail, "this binary is "+exeReal) {
		t.Fatalf("command row = %+v", cmd)
	}
	if cmd.fix != mcpSetupFix {
		t.Errorf("command fix = %q, want %q", cmd.fix, mcpSetupFix)
	}
	// A mismatch still probes the registered command: it may well answer.
	if !probed {
		t.Error("a mismatched command should still be probed")
	}
	if got["mcp answers"].status != doctorOK {
		t.Errorf("answers row = %+v, want ok", got["mcp answers"])
	}
}

func TestCheckClaudeCodeMcpMatchAndAnswers(t *testing.T) {
	home, exe := mcpTestEnv(t)
	writeClaudeEntry(t, home, exe, true)
	got := collectMcp(envFor(home, exe, func(_ context.Context, command string) (mcpProbeResult, error) {
		if command != exe {
			t.Errorf("probe got command %q, want %q", command, exe)
		}
		return mcpProbeResult{Tools: 11}, nil
	}))
	if r := got["mcp registered"]; r.status != doctorOK {
		t.Errorf("registered = %+v", r)
	}
	if r := got["mcp command"]; r.status != doctorOK || !strings.Contains(r.detail, "this binary") {
		t.Errorf("command = %+v", r)
	}
	ans := got["mcp answers"]
	if ans.status != doctorOK || !strings.Contains(ans.detail, "11 tools") {
		t.Errorf("answers = %+v", ans)
	}
}

func TestCheckClaudeCodeMcpAnswersFailureWarns(t *testing.T) {
	home, exe := mcpTestEnv(t)
	writeClaudeEntry(t, home, exe, true)
	got := collectMcp(envFor(home, exe, func(context.Context, string) (mcpProbeResult, error) {
		return mcpProbeResult{}, errors.New("boom: exit 1")
	}))
	ans := got["mcp answers"]
	if ans.status != doctorWarn || !strings.Contains(ans.detail, "boom") || ans.fix != mcpSetupFix {
		t.Errorf("answers row = %+v", ans)
	}
}

func TestCheckClaudeCodeMcpUnresolvableCommandSkipsProbe(t *testing.T) {
	home, exe := mcpTestEnv(t)
	writeClaudeEntry(t, home, filepath.Join(home, "bin", "gone-downshift"), true)
	got := collectMcp(envFor(home, exe, func(context.Context, string) (mcpProbeResult, error) {
		t.Fatal("probe must not run when the command does not resolve")
		return mcpProbeResult{}, nil
	}))
	cmd := got["mcp command"]
	if cmd.status != doctorWarn || !strings.Contains(cmd.detail, "does not resolve") {
		t.Fatalf("command row = %+v", cmd)
	}
	if _, ok := got["mcp answers"]; ok {
		t.Error("answers row should be skipped for an unresolvable command")
	}
}

// fakeServerScript writes a shell shim that behaves enough like the real MCP
// server for probeMcpTools: it reads the newline-delimited requests on stdin,
// prints a reply with n tools for tools/list (id 2) and a valid tool result for
// tools/call (id 3), then exits. Tests use it so the real claude CLI and a real
// server are never needed.
// fakeServerDeadline guards a probe of a fake server, which answers or exits at
// once. It is not the production mcpProbeTimeout: these tests check what the
// probe reads, and a loaded machine (parallel worktrees, make check) can take
// longer than 2 s just to start a shell. Tests of the deadline itself set their own.
const fakeServerDeadline = 15 * time.Second

func fakeServerScript(t *testing.T, n int) string {
	t.Helper()
	return fakeServerWithCall(t, n, `{"content":[{"type":"text","text":"{\"playbooks\":[]}"}]}`)
}

// fakeServerWithCall is fakeServerScript with the tools/call result chosen by
// the test (the JSON after "result":). An empty call result leaves tools/call
// unanswered and the shim exits, as a server that crashes on the call would.
func fakeServerWithCall(t *testing.T, n int, callResult string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake server shim is a /bin/sh script")
	}
	path := filepath.Join(t.TempDir(), "fake-downshift")
	toolsJSON := strings.Repeat(`{"name":"t"},`, n)
	toolsJSON = strings.TrimSuffix(toolsJSON, ",")
	callReply := ""
	if callResult != "" {
		reply := `{"jsonrpc":"2.0","id":3,"result":` + callResult + `}`
		callReply = `printf '%s\n' '` + strings.ReplaceAll(reply, `'`, `'\''`) + `'`
	}
	// The shim ignores initialize and the notification. It expects `mcp serve`
	// as its arguments, like the real binary.
	script := `#!/bin/sh
[ "$1" = "mcp" ] && [ "$2" = "serve" ] || { echo "usage" >&2; exit 2; }
while IFS= read -r line; do
  case "$line" in
    *'"tools/list"'*)
      printf '{"jsonrpc":"2.0","id":2,"result":{"tools":[` + toolsJSON + `]}}\n'
      ;;
    *'"tools/call"'*)
      ` + callReply + `
      exit 0
      ;;
  esac
done
exit 0
`
	writeFile(t, path, script)
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestProbeMcpToolsAgainstFakeServer(t *testing.T) {
	script := fakeServerScript(t, 7)
	ctx, cancel := context.WithTimeout(context.Background(), testenv.Scaled(fakeServerDeadline))
	defer cancel()
	res, err := probeMcpTools(ctx, script)
	if err != nil {
		t.Fatalf("probe error: %v", err)
	}
	if res.Tools != 7 || res.CallErr != nil {
		t.Errorf("probe = %+v, want 7 tools and a valid call", res)
	}
}

func TestProbeMcpToolsTimesOutOnSilentServer(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the silent server shim is a /bin/sh script")
	}
	// A server that reads forever and never answers: the probe's timeout must
	// end it, and the child must be killed (CommandContext + Kill on return).
	path := filepath.Join(t.TempDir(), "silent-downshift")
	writeFile(t, path, "#!/bin/sh\ncat >/dev/null\n")
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*1000*1000) // 300ms
	defer cancel()
	if _, err := probeMcpTools(ctx, path); err == nil {
		t.Error("a silent server should make the probe fail")
	}
}

// stubDoctorMcpCheck replaces the MCP check seam with a no-op for the duration
// of a test, so a buildDoctorReport fixture is not perturbed by the real
// machine's ~/.claude.json or a spawned server. The original is restored.
func stubDoctorMcpCheck(t *testing.T) {
	t.Helper()
	prev := doctorMcpCheck
	doctorMcpCheck = func(func(id, status, detail, fix string), string) {}
	t.Cleanup(func() { doctorMcpCheck = prev })
}

// TestDoctorMcpSeamRunsForClaudeCodeOnly proves the harness check calls the
// MCP seam exactly for claude-code: a full doctor report invokes it once with
// "claude-code" and never with another harness.
func TestDoctorMcpSeamRunsForClaudeCode(t *testing.T) {
	f := newDoctorFixture(t) // stubs the seam; we override it again below
	writeFile(t, filepath.Join(f.home, ".claude", "settings.json"), claudeSettings(f.bin))

	var harnesses []string
	prev := doctorMcpCheck
	doctorMcpCheck = func(add func(id, status, detail, fix string), harness string) {
		harnesses = append(harnesses, harness)
	}
	t.Cleanup(func() { doctorMcpCheck = prev })

	buildDoctorReport(f.env)

	for _, h := range harnesses {
		if h != "claude-code" {
			t.Errorf("MCP seam called for %q, want claude-code only", h)
		}
	}
	if len(harnesses) == 0 {
		t.Error("MCP seam was never called for the claude-code harness")
	}
}

// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const claudeAddLine = "claude mcp add -s user downshift -e DOWNSHIFT_MANAGED=setup -- " + testBin + " mcp serve"

// fakeMcpClaude stands in for the claude CLI: it records the calls and, like
// the real one, edits ~/.claude.json on `mcp add` and `mcp remove`.
type fakeMcpClaude struct {
	e       *setupEnv
	have    bool
	calls   []string
	failAdd bool
}

func (f *fakeMcpClaude) exec() setupExec {
	return setupExec{
		LookPath: func(name string) (string, error) {
			if name == "claude" && f.have {
				return "/usr/local/bin/claude", nil
			}
			return "", errors.New("not found")
		},
		Run: func(name string, args ...string) ([]byte, error) {
			f.calls = append(f.calls, "claude "+strings.Join(args, " "))
			doc := map[string]any{}
			if raw, err := os.ReadFile(f.e.path(".claude.json")); err == nil {
				_ = json.Unmarshal(raw, &doc)
			}
			servers, _ := doc["mcpServers"].(map[string]any)
			if servers == nil {
				servers = map[string]any{}
			}
			switch {
			case len(args) > 1 && args[1] == "add":
				if f.failAdd {
					return []byte("boom: cannot write\n"), errors.New("exit 1")
				}
				sep := 0
				for i, a := range args {
					if a == "--" {
						sep = i
					}
				}
				servers["downshift"] = map[string]any{"type": "stdio", "command": args[sep+1], "args": args[sep+2:],
					"env": map[string]any{"DOWNSHIFT_MANAGED": strings.TrimPrefix(args[sep-1], "DOWNSHIFT_MANAGED=")}}
			case len(args) > 1 && args[1] == "remove":
				delete(servers, "downshift")
			}
			doc["mcpServers"] = servers
			out, _ := json.Marshal(doc)
			return nil, os.WriteFile(f.e.path(".claude.json"), out, 0o600)
		},
	}
}

// mcpRun runs setup with the fake CLI, never the plugin path.
func (e *setupEnv) mcpRun(f *fakeMcpClaude, args ...string) (code int, out, errOut string) {
	e.t.Helper()
	var o, er bytes.Buffer
	// Mcp starts as the CLI default (on); --no-mcp turns it off, as in runSetup.
	opts := setupOptions{Home: e.home, Binary: testBin, SkipDiscovery: true, NoPlugin: true, Mcp: true,
		In: strings.NewReader(""), Out: &o, Err: &er, Exec: f.exec()}
	opts.Now = func() time.Time {
		e.tick++
		return time.Date(2026, 10, 28, 12, 0, 0, e.tick*1000, time.UTC)
	}
	for _, a := range args {
		switch a {
		case "--dry-run":
			opts.DryRun = true
		case "--undo":
			opts.Undo = true
		case "--yes":
			opts.Yes = true
		case "--mcp":
			opts.Mcp = true
		case "--no-mcp":
			opts.Mcp = false
		default:
			opts.Names = append(opts.Names, a)
		}
	}
	return setupRun(opts), o.String(), er.String()
}

func (e *setupEnv) claudeServers() map[string]any {
	e.t.Helper()
	m := parseDoc(e.t, e.read(".claude.json"))
	s, _ := m["mcpServers"].(map[string]any)
	return s
}

const codexOriginal = "# my codex config\nmodel = \"x\"   # keep me\n\n[mcp_servers.other]\ncommand = \"other\"\nargs = [\"a\"]\n"

func newMcpEnv(t *testing.T) (*setupEnv, *fakeMcpClaude) {
	e := newSetupEnv(t, ".claude", ".codex")
	e.write(".codex/config.toml", codexOriginal)
	e.write(".claude.json", `{"numStartups": 3, "mcpServers": {"other": {"type": "stdio", "command": "other", "args": []}}}`)
	return e, &fakeMcpClaude{e: e, have: true}
}

func TestSetupMcpIsOnByDefaultAndNoMcpSkipsIt(t *testing.T) {
	e, f := newMcpEnv(t)
	if code, _, errOut := e.mcpRun(f, "claude-code", "codex", "--no-mcp", "--yes"); code != 0 {
		t.Fatalf("code %d: %s", code, errOut)
	}
	if len(f.calls) != 0 {
		t.Errorf("claude ran with --no-mcp: %v", f.calls)
	}
	if got := e.read(".codex/config.toml"); got != codexOriginal {
		t.Errorf("config.toml changed with --no-mcp:\n%s", got)
	}
	if code, _, errOut := e.mcpRun(f, "claude-code", "codex", "--yes"); code != 0 {
		t.Fatalf("default run: code %d: %s", code, errOut)
	}
	if len(f.calls) == 0 || !strings.Contains(e.read(".codex/config.toml"), "[mcp_servers.downshift]") {
		t.Errorf("default run did not register: calls=%v", f.calls)
	}
}

func TestSetupMcpDryRunWritesNothing(t *testing.T) {
	e, f := newMcpEnv(t)
	before := e.read(".claude.json")
	code, out, errOut := e.mcpRun(f, "claude-code", "codex", "--mcp", "--dry-run")
	if code != 0 {
		t.Fatalf("code %d: %s", code, errOut)
	}
	if len(f.calls) != 0 || e.read(".claude.json") != before || e.read(".codex/config.toml") != codexOriginal {
		t.Errorf("dry run wrote: calls=%v", f.calls)
	}
	for _, want := range []string{"would run: " + claudeAddLine, "+# downshift:mcp:begin", "Dry run: nothing was written."} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestSetupMcpFirstRunThenIdempotent(t *testing.T) {
	e, f := newMcpEnv(t)
	code, out, errOut := e.mcpRun(f, "claude-code", "codex", "--mcp", "--yes")
	if code != 0 {
		t.Fatalf("code %d: %s", code, errOut)
	}
	if len(f.calls) != 1 || f.calls[0] != claudeAddLine {
		t.Fatalf("calls = %v\n%s\n%s", f.calls, out, errOut)
	}
	servers := e.claudeServers()
	if servers["downshift"] == nil || servers["other"] == nil {
		t.Errorf("servers after add: %v", servers)
	}
	toml := e.read(".codex/config.toml")
	if !strings.HasPrefix(toml, codexOriginal) || !strings.Contains(toml, "[mcp_servers.downshift]\ncommand = \""+testBin+"\"\nargs = [\"mcp\", \"serve\"]") {
		t.Errorf("config.toml:\n%s", toml)
	}
	if !strings.Contains(out, "registered") {
		t.Errorf("no registration line:\n%s", out)
	}
	code, out, errOut = e.mcpRun(f, "claude-code", "codex", "--mcp", "--yes")
	if code != 0 || len(f.calls) != 1 || e.read(".codex/config.toml") != toml {
		t.Fatalf("second run not idempotent: code=%d calls=%v err=%s", code, f.calls, errOut)
	}
	if strings.Count(out, "already up to date") < 2 {
		t.Errorf("second run should report both as up to date:\n%s", out)
	}
}

// A conflict is a warning on every harness: setup goes on, exits 0 and leaves
// the other entry as it is.
func TestSetupMcpConflictsAreWarnings(t *testing.T) {
	t.Run("claude-code", func(t *testing.T) {
		e, f := newMcpEnv(t)
		conflict := `{"mcpServers": {"downshift": {"type": "stdio", "command": "/elsewhere/ds", "args": ["serve"]}}}`
		e.write(".claude.json", conflict)
		code, _, errOut := e.mcpRun(f, "claude-code", "--yes")
		if code != 0 || !strings.Contains(errOut, "warning:") || !strings.Contains(errOut, `"downshift"`) || len(f.calls) != 0 || e.read(".claude.json") != conflict {
			t.Errorf("code=%d calls=%v err=%s", code, f.calls, errOut)
		}
	})
	t.Run("codex", func(t *testing.T) {
		e, f := newMcpEnv(t)
		own := codexOriginal + "\n[mcp_servers.downshift]\ncommand = \"mine\"\n"
		e.write(".codex/config.toml", own)
		code, _, errOut := e.mcpRun(f, "codex", "--yes")
		if code != 0 || !strings.Contains(errOut, "warning:") || !strings.Contains(errOut, "mcp_servers.downshift") || e.read(".codex/config.toml") != own {
			t.Errorf("code=%d err=%s", code, errOut)
		}
	})
}

func TestSetupMcpUndoRemovesOnlyOurs(t *testing.T) {
	e, f := newMcpEnv(t)
	if code, _, errOut := e.mcpRun(f, "claude-code", "codex", "--mcp", "--yes"); code != 0 {
		t.Fatalf("install: %d %s", code, errOut)
	}
	code, out, errOut := e.mcpRun(f, "claude-code", "codex", "--undo", "--yes")
	if code != 0 {
		t.Fatalf("undo: %d %s\n%s", code, errOut, out)
	}
	if got := e.read(".codex/config.toml"); got != codexOriginal {
		t.Errorf("codex file not restored byte for byte:\n%q", got)
	}
	servers := e.claudeServers()
	if servers["downshift"] != nil || servers["other"] == nil {
		t.Errorf("claude servers after undo: %v", servers)
	}
	if last := f.calls[len(f.calls)-1]; last != "claude mcp remove -s user downshift" {
		t.Errorf("last call %q", last)
	}
}

func TestSetupMcpUndoLeavesForeignEntry(t *testing.T) {
	e, f := newMcpEnv(t)
	foreign := `{"mcpServers": {"downshift": {"type": "stdio", "command": "/x/ds", "args": ["other"]}}}`
	e.write(".claude.json", foreign)
	code, out, _ := e.mcpRun(f, "claude-code", "--undo", "--yes")
	if code != 0 || len(f.calls) != 0 || e.read(".claude.json") != foreign || !strings.Contains(out, "left as it is") {
		t.Errorf("code=%d calls=%v out=%s", code, f.calls, out)
	}
}

func TestSetupMcpClaudeMissingIsASkip(t *testing.T) {
	e, f := newMcpEnv(t)
	f.have = false
	code, out, errOut := e.mcpRun(f, "claude-code", "--mcp", "--yes")
	if code != 0 || !strings.Contains(out, "`claude` is not on PATH") || len(f.calls) != 0 {
		t.Errorf("code=%d out=%s err=%s", code, out, errOut)
	}
}

func TestSetupMcpCLIFailureIsReported(t *testing.T) {
	e, f := newMcpEnv(t)
	f.failAdd = true
	code, _, errOut := e.mcpRun(f, "claude-code", "--mcp", "--yes")
	if code != 1 || !strings.Contains(errOut, "boom") {
		t.Errorf("code=%d err=%s", code, errOut)
	}
}

func TestSetupMcpRefusesUnsafeBinary(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "downshift")
	if err := os.WriteFile(bin, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	o := setupOptions{Binary: bin, Mcp: true, Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
	if code := guardHookBinary(o, false, tmp); code != 1 {
		t.Errorf("guard code = %d, want 1 for a binary under the temp dir", code)
	}
	if code := guardHookBinary(o, true, tmp); code != 0 {
		t.Errorf("--allow-unsafe-binary should override, got %d", code)
	}
}

func TestSetupUsageMentionsMcp(t *testing.T) {
	var out, errw bytes.Buffer
	if code := runSetup([]string{"-h"}, strings.NewReader(""), &out, &errw); code != 0 || !strings.Contains(out.String(), "--no-mcp") {
		t.Errorf("usage lacks --no-mcp: %d\n%s", code, out.String())
	}
}

func TestSetupArgsMcpIsOnUnlessNoMcp(t *testing.T) {
	in := strings.NewReader("")
	for _, tc := range []struct {
		args []string
		want bool
	}{
		{nil, true},
		{[]string{"--no-mcp"}, false},
		{[]string{"--mcp"}, true},
		{[]string{"--mcp", "--no-mcp"}, false},
		{[]string{"--no-mcp", "--mcp"}, true},
	} {
		opts := defaultSetupOptions(in, io.Discard, io.Discard)
		if _, _, bad := parseSetupArgs(tc.args, &opts); bad != "" {
			t.Fatalf("%v: unexpected bad flag %q", tc.args, bad)
		}
		if opts.Mcp != tc.want {
			t.Errorf("%v: Mcp = %v, want %v", tc.args, opts.Mcp, tc.want)
		}
	}
}

func TestSetupArgsReportsUnknownFlagAndHelp(t *testing.T) {
	opts := defaultSetupOptions(strings.NewReader(""), io.Discard, io.Discard)
	if _, _, bad := parseSetupArgs([]string{"--bogus"}, &opts); bad != "--bogus" {
		t.Errorf("bad = %q, want --bogus", bad)
	}
	if _, help, _ := parseSetupArgs([]string{"--help"}, &opts); !help {
		t.Error("--help not reported as help")
	}
}

func TestSetupMcpUndoRemovesAFileSetupCreated(t *testing.T) {
	e := newSetupEnv(t, ".codex")
	f := &fakeMcpClaude{e: e}
	if code, _, errOut := e.mcpRun(f, "codex", "--mcp", "--yes"); code != 0 {
		t.Fatalf("install: %d %s", code, errOut)
	}
	if !strings.Contains(e.read(".codex/config.toml"), "[mcp_servers.downshift]") {
		t.Fatal("block not written to a new config.toml")
	}
	if code, _, errOut := e.mcpRun(f, "codex", "--undo", "--yes"); code != 0 {
		t.Fatalf("undo: %d %s", code, errOut)
	}
	if _, err := os.Stat(e.path(".codex/config.toml")); !os.IsNotExist(err) {
		t.Errorf("config.toml should be gone after undo, stat err = %v", err)
	}
}

func TestSetupMcpUnbalancedMarkersAreRefused(t *testing.T) {
	e := newSetupEnv(t, ".codex")
	broken := "a = 1\n" + mcpBlockBegin + "\n"
	e.write(".codex/config.toml", broken)
	code, _, errOut := e.mcpRun(&fakeMcpClaude{e: e}, "codex", "--mcp", "--yes")
	if code != 0 || !strings.Contains(errOut, "unbalanced") || e.read(".codex/config.toml") != broken {
		t.Errorf("code=%d err=%s", code, errOut)
	}
}

func TestSetupMcpTomlDuplicateFormsAreConflicts(t *testing.T) {
	forms := map[string]string{
		"header":        "[mcp_servers.downshift]\ncommand = \"mine\"\n",
		"dotted key":    "mcp_servers.downshift.command = \"mine\"\n",
		"dotted inline": "mcp_servers.downshift = { command = \"mine\" }\n",
		"in table":      "[mcp_servers]\ndownshift = { command = \"mine\" }\n",
		"root inline":   "mcp_servers = { downshift = { command = \"mine\" } }\n",
	}
	for name, body := range forms {
		t.Run(name, func(t *testing.T) {
			e := newSetupEnv(t, ".codex")
			e.write(".codex/config.toml", body)
			code, _, errOut := e.mcpRun(&fakeMcpClaude{e: e}, "codex", "--mcp", "--yes")
			if code != 0 || !strings.Contains(errOut, "already has") || e.read(".codex/config.toml") != body {
				t.Errorf("code=%d err=%s", code, errOut)
			}
		})
	}
}

func TestSetupMcpUndoNeedsTheSetupMarker(t *testing.T) {
	e, f := newMcpEnv(t)
	unmarked := `{"mcpServers": {"downshift": {"type": "stdio", "command": "/evil/bin", "args": ["mcp", "serve"], "env": {"K": "v"}}}}`
	e.write(".claude.json", unmarked)
	code, out, _ := e.mcpRun(f, "claude-code", "--undo", "--yes")
	if code != 0 || len(f.calls) != 0 || e.read(".claude.json") != unmarked || !strings.Contains(out, "was not registered by setup") {
		t.Errorf("code=%d calls=%v out=%s", code, f.calls, out)
	}
	// With the marker (and the binary setup would add) the entry is ours.
	if code, _, errOut := e.mcpRun(f, "claude-code", "--yes"); code != 0 || len(f.calls) != 0 || e.read(".claude.json") != unmarked {
		t.Errorf("an unmarked entry must be a warning and stay: code=%d calls=%v err=%s", code, f.calls, errOut)
	}
}

func TestSetupMcpDiffNeverShowsNeighbouringLines(t *testing.T) {
	e := newSetupEnv(t, ".codex")
	secret := "[mcp_servers.other]\ncommand = \"o\"\nenv = { TOKEN = \"s3cret-value\" }\n"
	e.write(".codex/config.toml", secret)
	f := &fakeMcpClaude{e: e}
	for _, args := range [][]string{{"--mcp", "--dry-run"}, {"--mcp", "--yes"}, {"--undo", "--dry-run"}} {
		_, out, errOut := e.mcpRun(f, append([]string{"codex"}, args...)...)
		if strings.Contains(out+errOut, "s3cret-value") {
			t.Errorf("%v leaked a neighbouring line:\n%s", args, out)
		}
	}
	if !strings.Contains(e.read(".codex/config.toml"), "s3cret-value") {
		t.Error("the user's line was lost")
	}
}

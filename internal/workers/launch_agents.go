// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// ReadOnlyLaunchers returns the launcher of each agent that has a read-only
// recipe. Claude runs in print mode under the planning permission mode; Codex
// runs in exec mode under the read-only sandbox. Both write machine-readable
// output so the job can read its token usage and its session id: Claude's
// --output-format json result object, Codex's --json event lines (its report
// still comes from -o).
func ReadOnlyLaunchers() map[string]Launcher {
	return map[string]Launcher{
		"claude": cliLauncher{plan: claudeReadOnly},
		"codex":  cliLauncher{plan: codexReadOnly},
	}
}

// ByAgent sends each job to the launcher of its agent. An agent with no
// launcher is refused as unverified, the same as the default launcher.
func ByAgent(launchers map[string]Launcher) Launcher { return byAgent(launchers) }

type byAgent map[string]Launcher

func (m byAgent) Launch(spec Spec) (Process, error) {
	l, ok := m[spec.Agent]
	if !ok {
		return nil, ErrNoVerifiedMode
	}
	return l.Launch(spec)
}

// claudeIsolation keeps a worker from loading the user's MCP servers. Without
// it a worker inherits every server the user registered, claude.ai connectors
// (mail, GitHub, documents) and Downshift itself included: under
// bypassPermissions those tools run without approval, and their definitions
// cost context on every call (measured 2026-10-31: 31 servers and 323 tools
// inherited, none with the flag).
var claudeIsolation = []string{"--strict-mcp-config"}

// A worker's session is now persisted so a later turn can resume the same
// conversation without re-paying to re-read its context (the spike's measured
// win). The recipes no longer pass --no-session-persistence (claude) or
// --ephemeral (codex); each launcher captures the session id its CLI writes and
// knows how to build the resume args for that CLI. The isolation never widened:
// --strict-mcp-config, the plan/bypass permission modes, the read-only/
// workspace-write sandboxes and the worktree-only writes are all unchanged.
//
// The persisted transcript is keyed by the working directory. A read worker
// runs in the directory the caller named (the project directory by default);
// its transcript lands in that project's folder and so would show in the user's
// own resume list there (mitigation is deletion on release, task T5 of
// docs/internal/plans/mcp-worker-continuation.md, not done here). A write worker
// runs in its own worktree under .downshift/worktrees/<id>, a directory the user
// never opens interactively, so its transcript lands in a separate project
// folder and does not show in the user's resume list in their own directory.

func claudeReadOnly(spec Spec) (argv, error) {
	args := append([]string{"-p", "--output-format", "json", "--permission-mode", "plan"}, claudeIsolation...)
	args = appendClaudeTuning(args, spec)
	return argv{name: "claude", args: args, report: claudeReport, tokens: claudeTokens, session: claudeSession, resume: claudeResumeArgs}, nil
}

// appendClaudeTuning adds the model and effort flags a spec carries, shared by
// the read-only and write recipes so both stay identical but for the permission
// mode.
func appendClaudeTuning(args []string, spec Spec) []string {
	if spec.Model != "" {
		args = append(args, "--model", spec.Model)
	}
	if spec.Effort != "" {
		args = append(args, "--effort", spec.Effort)
	}
	return args
}

// claudeResumeArgs are the args that continue a stored claude session with a
// new turn: `claude -p --output-format json --resume <id>` plus the same
// isolation. The permission mode, model and effort are added by the launcher
// from the original spec, so a resume never climbs a tier or widens a mode.
// T1 captures the id and builds these args; the tool that calls resume is a
// later task (T3). The id goes last so a malformed id cannot shift a flag.
func claudeResumeArgs(sessionID string) []string {
	args := append([]string{"-p", "--output-format", "json"}, claudeIsolation...)
	return append(args, "--resume", sessionID)
}

// claudeReport reads the single JSON object print mode writes: its result text,
// or the reason it did not succeed.
func claudeReport(stdout []byte) (string, error) {
	var r struct {
		Result  string `json:"result"`
		IsError bool   `json:"is_error"`
		Subtype string `json:"subtype"`
	}
	if err := json.Unmarshal(stdout, &r); err != nil {
		return "", fmt.Errorf("claude wrote no JSON result: %v", err)
	}
	if r.IsError || r.Subtype != "success" {
		return r.Result, fmt.Errorf("claude ended with %q", r.Subtype)
	}
	return r.Result, nil
}

// claudeSession reads the session id from the single JSON object print mode
// writes (the `session_id` field, confirmed in the installed CLI 2.1.316). It
// returns "" when the output carries no id (an older CLI, an error, or a
// malformed line), so a worker without a captured id simply is not resumable
// and nothing breaks.
func claudeSession(stdout []byte) string {
	var r struct {
		SessionID string `json:"session_id"`
	}
	if json.Unmarshal(bytes.TrimSpace(stdout), &r) != nil {
		return ""
	}
	return r.SessionID
}

func codexReadOnly(spec Spec) (argv, error) {
	f, err := os.CreateTemp("", "downshift-worker-*.txt")
	if err != nil {
		return argv{}, errors.New("cannot create the worker's report file")
	}
	path := f.Name()
	_ = f.Close()
	args := []string{"exec", "--sandbox", "read-only", "--skip-git-repo-check", "--json", "-C", spec.Dir, "-o", path}
	args = appendCodexTuning(args, spec)
	args = append(args, "-")
	report := func([]byte) (string, error) {
		b, err := os.ReadFile(path)
		if err != nil {
			return "", errors.New("codex wrote no final message")
		}
		return string(b), nil
	}
	return argv{name: "codex", args: args, report: report, tokens: codexTokens, session: codexSession, resume: codexResumeArgs, cleanup: func() { _ = os.Remove(path) }}, nil
}

// appendCodexTuning adds the model and effort a spec carries, shared by the
// read-only and write recipes.
func appendCodexTuning(args []string, spec Spec) []string {
	if spec.Model != "" {
		args = append(args, "--model", spec.Model)
	}
	if spec.Effort != "" {
		args = append(args, "-c", "model_reasoning_effort="+spec.Effort)
	}
	return args
}

// codexResumeArgs are the args that continue a stored codex session with a new
// turn: `codex exec resume <id>` with the prompt read from stdin, under the
// same read-only sandbox and JSON event stream. The sandbox, model and effort
// are added by the launcher from the original spec. T1 builds these args; the
// tool that calls resume is a later task (T3). The id goes right after `resume`
// as its positional argument.
func codexResumeArgs(sessionID string) []string {
	return []string{"exec", "resume", sessionID, "--sandbox", "read-only", "--skip-git-repo-check", "--json", "-"}
}

// codexSession reads the thread id codex prints on its thread.started event
// (the `thread_id` field of `codex exec --json`, confirmed in codex-cli
// 0.163.0). It scans the JSON lines and returns the first thread id it sees, or
// "" when no line carries one (so a worker without an id is simply not
// resumable).
func codexSession(stdout []byte) string {
	sc := bufio.NewScanner(bytes.NewReader(stdout))
	sc.Buffer(make([]byte, 0, 64*1024), maxOutput*4)
	for sc.Scan() {
		var ev struct {
			Type     string `json:"type"`
			ThreadID string `json:"thread_id"`
		}
		if json.Unmarshal(sc.Bytes(), &ev) != nil {
			continue
		}
		if ev.Type == "thread.started" && ev.ThreadID != "" {
			return ev.ThreadID
		}
	}
	return ""
}

// claudeWrite is the write recipe: the bypass permission mode, inside a worktree
// that writeIsolated prepared.
func claudeWrite(spec Spec) (argv, error) {
	args := append([]string{"-p", "--output-format", "json", "--permission-mode", "bypassPermissions"}, claudeIsolation...)
	args = appendClaudeTuning(args, spec)
	return argv{name: "claude", args: args, report: claudeReport, tokens: claudeTokens, session: claudeSession, resume: claudeResumeArgs}, nil
}

// codexWrite is the write recipe: workspace-write sandbox, approval never.
func codexWrite(spec Spec) (argv, error) {
	a, err := codexReadOnly(spec)
	if err != nil {
		return argv{}, err
	}
	for i, arg := range a.args {
		if arg == "read-only" {
			a.args[i] = "workspace-write"
		}
	}
	a.args = append([]string{"exec", "-a", "never"}, a.args[1:]...)
	return a, nil
}

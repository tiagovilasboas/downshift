// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"os"
)

// codexApprovalNever is the never-ask approval policy as a -c override. `codex
// exec` has no -a/--ask-for-approval flag (only the top-level `codex` does;
// codex-cli 0.163.0 rejects it), so launch and resume both ride on this one.
const codexApprovalNever = `approval_policy="never"`

func codexReadOnly(spec Spec) (argv, error) {
	return codexArgv("read-only", false, spec)
}

// codexArgv builds the Codex recipe shared by the read-only and write modes,
// differing in the sandbox and, for write, the never-ask approval policy
// (a -c override, since `codex exec` has no -a flag).
func codexArgv(sandbox string, approveNever bool, spec Spec) (argv, error) {
	f, err := os.CreateTemp("", "downshift-worker-*.txt")
	if err != nil {
		return argv{}, errors.New("cannot create the worker's report file")
	}
	path := f.Name()
	_ = f.Close()
	args := []string{"exec"}
	if approveNever {
		args = append(args, "-c", codexApprovalNever)
	}
	args = append(args, "--sandbox", sandbox, "--skip-git-repo-check", "--json", "-C", spec.Dir, "-o", path)
	args = append(args, codexMCPOff...)
	args = appendCodexTuning(args, spec)
	args = append(args, "-")
	report := func([]byte) (string, error) {
		b, err := os.ReadFile(path)
		if err != nil {
			return "", errors.New("codex wrote no final message")
		}
		return string(b), nil
	}
	resume := func(sessionID string) []string { return codexResumeArgs(sandbox, approveNever, spec, path, sessionID) }
	return argv{name: "codex", args: args, report: report, tokens: codexTokens, session: codexSession, resume: resume, cleanSession: codexCleanSession, cleanup: func() { _ = os.Remove(path) }}, nil
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
// turn: `codex exec resume <id>` with the prompt read from stdin. The installed
// `exec resume` has no --sandbox or -a flag (codex-cli 0.163.0 rejects them), so
// the sandbox and approval policy ride on -c overrides; the model and effort
// match the first launch and MCP servers stay off. -o names the report file
// the turn's report reads, as on the first launch; without it the resumed turn
// delivers the raw event stream. The id goes right after `resume` as its
// positional argument.
func codexResumeArgs(sandbox string, approveNever bool, spec Spec, out, sessionID string) []string {
	args := []string{"exec", "resume", sessionID, "-c", "sandbox_mode=" + `"` + sandbox + `"`}
	if approveNever {
		args = append(args, "-c", codexApprovalNever)
	}
	args = append(args, "--skip-git-repo-check", "--json", "-o", out)
	args = append(args, codexMCPOff...)
	args = appendCodexTuning(args, spec)
	return append(args, "-")
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

// codexWrite is the write recipe: workspace-write sandbox, approval never.
func codexWrite(spec Spec) (argv, error) {
	return codexArgv("workspace-write", true, spec)
}

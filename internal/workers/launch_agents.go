// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// ReadOnlyLaunchers returns the launcher of each agent that has a read-only
// recipe. Claude runs in print mode under the planning permission mode; Codex
// runs in exec mode under the read-only sandbox.
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

func claudeReadOnly(spec Spec) (argv, error) {
	args := []string{"-p", "--output-format", "json", "--permission-mode", "plan", "--no-session-persistence"}
	if spec.Model != "" {
		args = append(args, "--model", spec.Model)
	}
	if spec.Effort != "" {
		args = append(args, "--effort", spec.Effort)
	}
	return argv{name: "claude", args: args, report: claudeReport}, nil
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

func codexReadOnly(spec Spec) (argv, error) {
	f, err := os.CreateTemp("", "downshift-worker-*.txt")
	if err != nil {
		return argv{}, errors.New("cannot create the worker's report file")
	}
	path := f.Name()
	_ = f.Close()
	args := []string{"exec", "--sandbox", "read-only", "--skip-git-repo-check", "--ephemeral", "-C", spec.Dir, "-o", path}
	if spec.Model != "" {
		args = append(args, "--model", spec.Model)
	}
	if spec.Effort != "" {
		args = append(args, "-c", "model_reasoning_effort="+spec.Effort)
	}
	args = append(args, "-")
	report := func([]byte) (string, error) {
		b, err := os.ReadFile(path)
		if err != nil {
			return "", errors.New("codex wrote no final message")
		}
		return string(b), nil
	}
	return argv{name: "codex", args: args, report: report, cleanup: func() { _ = os.Remove(path) }}, nil
}

// claudeWrite is the write recipe: the bypass permission mode, inside a worktree
// that writeIsolated prepared.
func claudeWrite(spec Spec) (argv, error) {
	args := []string{"-p", "--output-format", "json", "--permission-mode", "bypassPermissions", "--no-session-persistence"}
	if spec.Model != "" {
		args = append(args, "--model", spec.Model)
	}
	if spec.Effort != "" {
		args = append(args, "--effort", spec.Effort)
	}
	return argv{name: "claude", args: args, report: claudeReport}, nil
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

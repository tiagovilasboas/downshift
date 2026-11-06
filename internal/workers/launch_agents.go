// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"errors"
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

// LaunchResume continues a worker's session with the launcher of its agent, so
// a ByAgent queue is resumable whenever the agent's launcher is. An agent with
// no launcher, or one whose launcher cannot resume, is refused the same way the
// launch is: the queue never resumes a turn it cannot start. This keeps the
// resume agnostic at the queue, with the per-CLI recipe inside each launcher.
func (m byAgent) LaunchResume(spec Spec, sessionID string) (Process, error) {
	if !validSessionID(sessionID) {
		return nil, errBadSessionID
	}
	l, ok := m[spec.Agent]
	if !ok {
		return nil, ErrNoVerifiedMode
	}
	rl, ok := l.(resumeLauncher)
	if !ok {
		return nil, errors.New(spec.Agent + " cannot resume a session")
	}
	return rl.LaunchResume(spec, sessionID)
}

// A worker's session is persisted so a later turn can resume the same
// conversation without re-paying to re-read its context. Each launcher captures
// the session id its CLI writes and builds the resume args for that CLI, with
// the first launch's mode, sandbox, model and effort. The isolation never widened:
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

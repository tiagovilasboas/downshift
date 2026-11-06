// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
)

// WorkerMarkerEnv is the environment variable set to 1 in every worker process.
const WorkerMarkerEnv = "DOWNSHIFT_WORKER"

// startProcess resolves the program on PATH and starts it in the spec's
// directory with the prompt on standard input, returning a cliProcess. It is
// the shared tail of Launch and LaunchResume so both build the same kind of
// process from an argv; its cleanup runs if the program is missing or the start
// fails.
func startProcess(a argv, spec Spec) (Process, error) {
	path, err := exec.LookPath(a.name)
	if err != nil {
		if a.cleanup != nil {
			a.cleanup()
		}
		return nil, fmt.Errorf("%s is not on PATH", a.name)
	}
	cmd := exec.Command(path, a.args...)
	cmd.Dir = spec.Dir
	// Every worker carries the marker so Downshift's own tools can tell they run
	// inside a worker and refuse to delegate again. The parent environment is
	// filtered to what the CLI needs (env.go), so unrelated tokens stay out.
	cmd.Env = append(workerEnv(os.Environ()), spec.Env...)
	cmd.Env = append(cmd.Env, WorkerMarkerEnv+"=1")
	cmd.Stdin = strings.NewReader(spec.Task)
	out := NewTail(maxOutput * 4)
	head := NewHead(sessionHeadBytes)
	errTail := NewTail(4096)
	cmd.Stdout = io.MultiWriter(out, head)
	cmd.Stderr = errTail
	ownProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		releaseProcess(cmd)
		if a.cleanup != nil {
			a.cleanup()
		}
		return nil, fmt.Errorf("start %s: %v", a.name, err)
	}
	adoptProcess(cmd)
	return &cliProcess{cmd: cmd, out: out, head: head, errTail: errTail, argv: a}, nil
}

// sessionHeadBytes is how much of the start of a worker's output is kept apart
// from the tail. Codex prints its thread id on the first event, and a long run
// writes far more than the tail keeps, so without the head that id is gone by
// the time the worker exits and the worker can be neither resumed nor cleaned.
const sessionHeadBytes = 8 * 1024

// cliProcess is a started worker.
type cliProcess struct {
	cmd     *exec.Cmd
	out     *Tail
	head    *Head
	errTail *Tail
	argv    argv
	report  string
	// tokens and session are set by Wait and read by the queue under its own
	// lock, from another goroutine, so they are atomic. session holds the id the
	// CLI wrote, empty when it wrote none.
	tokens  atomic.Pointer[Tokens]
	session atomic.Pointer[string]
}

// Wait blocks until the worker exits, then reads its report. Its cleanup runs
// once, whatever the outcome.
func (p *cliProcess) Wait() error {
	err := p.cmd.Wait()
	releaseProcess(p.cmd)
	if p.argv.cleanup != nil {
		defer p.argv.cleanup()
	}
	stdout := []byte(p.out.String())
	report, rerr := p.argv.report(stdout)
	p.report = report
	if p.argv.tokens != nil {
		p.tokens.Store(p.argv.tokens(stdout))
	}
	if p.argv.session != nil {
		id := p.argv.session(stdout)
		if id == "" && p.head != nil {
			// The id line came first and the tail rolled past it.
			id = p.argv.session(p.head.Bytes())
		}
		if id != "" {
			p.session.Store(&id)
		}
	}
	if err != nil {
		return err
	}
	return rerr
}

// Kill stops the worker and the programs it started.
func (p *cliProcess) Kill() error {
	err := killGroup(p.cmd)
	if errors.Is(err, os.ErrProcessDone) {
		return nil
	}
	return err
}

// Report is the final report once the worker has exited, else the output so far.
func (p *cliProcess) Report() string {
	if p.report != "" {
		return p.report
	}
	return p.out.String()
}

// Stderr is the tail of the worker's standard error.
func (p *cliProcess) Stderr() string { return p.errTail.String() }

// Tokens is what the worker reported spending, nil before it exits or when its
// output carried no usage.
func (p *cliProcess) Tokens() *Tokens { return p.tokens.Load() }

// SessionID is the id this worker's session has. When the launcher knew the id
// upfront (Claude's generated --session-id), it is that preset and is available
// from the moment the worker starts, so a first-turn cancel has an id to delete
// the transcript with even though the killed CLI wrote no final JSON. Otherwise
// it is the id the CLI wrote, captured after the worker exits (empty before
// then, and empty when the CLI wrote none). The captured id, when present,
// equals the preset for Claude; it is the only source for a CLI with no upfront
// flag (Codex's thread id).
func (p *cliProcess) SessionID() string {
	if id := p.session.Load(); id != nil && *id != "" {
		return *id
	}
	return p.argv.presetSession
}

// PresetSessionID is the id the launcher knew before the process started, or ""
// when the CLI has no way to set it upfront. The queue reads it at start so a
// worker killed on its first turn still has an id for cleanup; a worker whose
// CLI assigns the id only at runtime reports "" and is resumable (and cleanable)
// only after it writes the id.
func (p *cliProcess) PresetSessionID() string { return p.argv.presetSession }

// ResumeArgs builds the args that continue this worker's stored session with a
// new turn, or nil when the recipe cannot resume. The args already carry the
// permission mode or sandbox, model and effort of the first launch.
func (p *cliProcess) ResumeArgs(sessionID string) []string {
	if p.argv.resume == nil {
		return nil
	}
	return p.argv.resume(sessionID)
}

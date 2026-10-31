// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync/atomic"
)

// The read-only launches. The recipe follows the public behaviour of other
// orchestrators (a print-mode run with the prompt on standard input, a planning
// permission mode for Claude, a read-only sandbox for Codex) and the flags
// shown by each CLI's own help. The Claude recipe has completed one real read
// run; the Codex recipe has not run against its CLI yet. The spec's R27 stays
// open until a write is seen refused under plan mode (docs/internal/plans/mcp-delegation-spec.md).

var (
	modelPattern  = regexp.MustCompile(`^[A-Za-z0-9._:/][A-Za-z0-9._:/-]{0,127}$`)
	effortPattern = regexp.MustCompile(`^[a-z]{1,16}$`)
)

// argv is what a launch runs: the program, its arguments, how to read the
// report, the token usage and the session id from its output, how to build the
// args that resume that session, and what to remove afterwards. session and
// resume are per-CLI, so the queue only ever holds the opaque id and never
// learns which agent produced it.
type argv struct {
	name   string
	args   []string
	report func(stdout []byte) (string, error)
	tokens func(stdout []byte) *Tokens
	// session reads the session id the CLI wrote, or "" when the output carried
	// none (an older CLI, an error, or a malformed line). nil means the recipe
	// does not persist a session.
	session func(stdout []byte) string
	// resume builds the args that continue a stored session with a new turn.
	// T1 captures the id and keeps this builder next to the launcher; the tool
	// that calls resume is a later task. nil means the recipe cannot resume.
	resume  func(sessionID string) []string
	cleanup func()
}

// planner builds the argv for one spec.
type planner func(spec Spec) (argv, error)

// cliLauncher starts a worker from a planner.
type cliLauncher struct {
	plan planner
}

// Launch validates the spec, resolves the program on PATH and starts it in the
// job's directory with the prompt on standard input.
func (l cliLauncher) Launch(spec Spec) (Process, error) {
	if err := checkSpec(spec); err != nil {
		return nil, err
	}
	a, err := l.plan(spec)
	if err != nil {
		return nil, err
	}
	path, err := exec.LookPath(a.name)
	if err != nil {
		if a.cleanup != nil {
			a.cleanup()
		}
		return nil, fmt.Errorf("%s is not on PATH", a.name)
	}
	cmd := exec.Command(path, a.args...)
	cmd.Dir = spec.Dir
	cmd.Stdin = strings.NewReader(spec.Task)
	out := NewTail(maxOutput * 4)
	errTail := NewTail(4096)
	cmd.Stdout = out
	cmd.Stderr = errTail
	ownProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		if a.cleanup != nil {
			a.cleanup()
		}
		return nil, fmt.Errorf("start %s: %v", a.name, err)
	}
	return &cliProcess{cmd: cmd, out: out, errTail: errTail, argv: a}, nil
}

// checkSpec refuses values that would change the program's meaning: a model or
// effort that starts with a dash becomes a flag, and a directory must exist.
func checkSpec(spec Spec) error {
	if spec.Model != "" && !modelPattern.MatchString(spec.Model) {
		return errors.New("model is not a plain model id")
	}
	if spec.Effort != "" && !effortPattern.MatchString(spec.Effort) {
		return errors.New("effort is not a plain level")
	}
	if spec.Dir == "" {
		return errors.New("a worker needs a directory")
	}
	info, err := os.Stat(spec.Dir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("%s is not an existing directory", spec.Dir)
	}
	return nil
}

// cliProcess is a started worker.
type cliProcess struct {
	cmd     *exec.Cmd
	out     *Tail
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
		if id := p.argv.session(stdout); id != "" {
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

// SessionID is the id the worker's CLI wrote, so a later turn can resume the
// same conversation. It is empty before the worker exits, and stays empty when
// the CLI wrote no id (an older CLI, an error, or a recipe that does not
// persist a session).
func (p *cliProcess) SessionID() string {
	if id := p.session.Load(); id != nil {
		return *id
	}
	return ""
}

// ResumeArgs builds the args that continue this worker's stored session with a
// new turn, or nil when the recipe cannot resume. The permission mode, model
// and effort a resume needs are added by the caller from the original spec, so
// a resumed turn never climbs a tier or widens a mode. T1 keeps this builder
// next to the launcher; the tool that calls resume is a later task.
func (p *cliProcess) ResumeArgs(sessionID string) []string {
	if p.argv.resume == nil {
		return nil
	}
	return p.argv.resume(sessionID)
}

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
// report from its output, and what to remove afterwards.
type argv struct {
	name    string
	args    []string
	report  func(stdout []byte) (string, error)
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
}

// Wait blocks until the worker exits, then reads its report. Its cleanup runs
// once, whatever the outcome.
func (p *cliProcess) Wait() error {
	err := p.cmd.Wait()
	if p.argv.cleanup != nil {
		defer p.argv.cleanup()
	}
	report, rerr := p.argv.report([]byte(p.out.String()))
	p.report = report
	if err != nil {
		return err
	}
	return rerr
}

// Kill stops the worker process.
func (p *cliProcess) Kill() error {
	if p.cmd.Process == nil {
		return nil
	}
	err := p.cmd.Process.Kill()
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

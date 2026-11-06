// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"errors"
	"fmt"
	"os"
	"regexp"
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
	name string
	args []string
	// presetSession is the session id the launcher generated and already put on
	// the args (Claude's --session-id), so the Downshift knows the id before the
	// process starts. Empty when the CLI has no way to set the id upfront (Codex)
	// or the RNG was unavailable, in which case the id is only known after the
	// run, from the CLI's output (the session reader).
	presetSession string
	report        func(stdout []byte) (string, error)
	tokens        func(stdout []byte) *Tokens
	// session reads the session id the CLI wrote, or "" when the output carried
	// none (an older CLI, an error, or a malformed line). nil means the recipe
	// does not persist a session.
	session func(stdout []byte) string
	// resume builds the args that continue a stored session with a new turn.
	// T1 captures the id and keeps this builder next to the launcher; the tool
	// that calls resume is a later task. nil means the recipe cannot resume.
	resume func(sessionID string) []string
	// cleanSession deletes the session this recipe's CLI persisted, given the
	// worker's directory and the opaque id. It lives next to the launcher so the
	// per-CLI deletion path stays with the CLI (cleanup.go). nil means the recipe
	// persists nothing to delete.
	cleanSession cleanSessionFunc
	cleanup      func()
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
	return startProcess(a, spec)
}

// LaunchResume continues a worker's stored session with a new turn. It builds
// the argv from the original spec (the same planner the launch used), then
// replaces the launch args with the resume args the planner carries for that
// CLI, keeping its report, token and session readers. The spec is the original
// one with only Task replaced, and the resume args carry the first launch's
// mode, sandbox, model and effort, so the resumed turn never widens its mode:
// the queue decides nothing about the CLI, the launcher owns the resume recipe.
// A planner whose recipe cannot resume (resume is nil) is refused, so the queue
// never starts a turn it cannot continue.
func (l cliLauncher) LaunchResume(spec Spec, sessionID string) (Process, error) {
	if err := checkSpec(spec); err != nil {
		return nil, err
	}
	a, err := l.plan(spec)
	if err != nil {
		return nil, err
	}
	if a.resume == nil {
		if a.cleanup != nil {
			a.cleanup()
		}
		return nil, errors.New(a.name + " cannot resume a session")
	}
	a.args = a.resume(sessionID)
	// A resumed turn continues the existing conversation: the preset is the id
	// it resumes, never the fresh one the planner generated for a first turn
	// (those args are replaced by the resume args). So the queue keeps the same
	// id across turns and a resume never opens a second session.
	a.presetSession = sessionID
	return startProcess(a, spec)
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

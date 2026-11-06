// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package workers runs the work an orchestrator delegates to other agent CLIs.
// A job is one worker process with its own budget; a queue starts as many as a
// limit allows and keeps each result until the caller acknowledges it.
//
// The package knows no harness: a Launcher starts a worker, and the default one
// refuses every agent until its read-only launch has been verified (spec R27).
// Behaviour follows docs/internal/plans/mcp-delegation-spec.md, sections 1 and 5.
//
// Downshift by Tiago Vilas Boas
// https://github.com/tiagovilasboas/downshift
package workers

import "time"

// State is where a job is.
type State string

const (
	Queued    State = "queued"
	Running   State = "running"
	Done      State = "done"
	Failed    State = "failed"
	Cancelled State = "cancelled"
	TimedOut  State = "timeout"
	// Blocked is a worker that paused mid-turn waiting on a question (a
	// permission request it can answer). The plan defines it as the hook the
	// deferred answer tool (T7) needs; no launch reaches it yet, so no job is in
	// Blocked today. It is not a finished state: a blocked worker is still live.
	Blocked State = "blocked"
)

// Finished reports whether a state ends the job. Queued, Running and Blocked
// are the live states; every other state has ended the job. A settled job may
// still be resumable (continuation.go), but Finished stays a question about the
// current turn, not about whether another turn can start.
func (s State) Finished() bool { return s != Queued && s != Running && s != Blocked }

// Spec is what one job asks for. Task is the prompt: it is held in memory for
// the life of the job and never written to disk (spec R6).
type Spec struct {
	// ID is the job id; the queue sets it. A write job's worktree is named after it.
	ID string
	// Write asks for the write route (a bypass in its own worktree); false is read-only.
	Write  bool
	Agent  string
	Model  string
	Effort string
	Dir    string
	Task   string
	Budget time.Duration
	// Env holds extra KEY=VALUE entries added to the worker process environment
	// (for example a gateway base URL). Empty leaves the environment unchanged.
	Env []string
}

// Status is a job as the caller sees it, without its prompt.
type Status struct {
	ID    string `json:"id"`
	Agent string `json:"agent"`
	State State  `json:"state"`
}

// job is the queue's record of one worker. Its fields are touched only under
// the queue's lock.
type job struct {
	id    string
	spec  Spec
	state State
	proc  Process
	timer *time.Timer
	// worktree and base are set when a write worker is launched in its own git
	// worktree: worktree is that directory, base is the commit it was created
	// from. Both empty for a read-only worker. They let Diff read the work so
	// far without a path from the caller.
	worktree string
	base     string
	// started is set when the job leaves the queue for a worker; ended is set
	// when the job finishes. Both are read under the lock to report elapsed time
	// in the status view, so it never touches the running process concurrently.
	started time.Time
	ended   time.Time
	// summary is a short, trimmed copy of the result or error, kept for the
	// status view after the full delivery has been acknowledged and dropped.
	summary string
	// tokens is the usage recorded at finish; nil while running or when the CLI
	// reported none. The status view reads this copy, never the live process.
	tokens *Tokens
	// sessionID is the opaque id the worker's CLI wrote, captured at finish so a
	// later turn can resume the same conversation. Empty while running, and
	// empty when the CLI wrote none (an older CLI, an error, or a recipe that
	// does not persist a session). The queue never reads what agent it belongs
	// to; resuming it is handled by the launcher (continuation.go).
	sessionID string
	// inbox holds the turns a caller queued for this worker while a turn was
	// running, in arrival order (continuation.go). The queue runs them one at a
	// time: when the live turn settles, the next inbox turn resumes the session.
	// It is touched only under the queue's lock, like every other job field.
	inbox []string
	// steering marks a worker whose live turn was interrupted by steer (steer.go)
	// so the correction can run first. Unlike cancel, the interrupt keeps the job
	// and its session resumable: when the killed process exits, wait settles the
	// job Done (not Failed) and the front-of-inbox steer resumes the session. It
	// is cleared once the job settles. Touched only under the queue's lock.
	steering bool
	// cleanupPending marks a worker whose on-disk session transcript must be
	// deleted once its process has truly exited. Cancel settles a worker at once
	// (it does not wait for the killed process to flush its output), so the
	// session id a killed turn writes on its way out is not yet known when Cancel
	// returns. The wait goroutine sets the id when the process exits and then,
	// seeing this flag, runs the deletion. It is touched only under the lock.
	cleanupPending bool
	// resume marks a queued job whose next start is a resumed turn of its
	// session, not a first launch: a follow-up waits for a slot like any job
	// (continuation.go). turn counts the turns started, so the wait goroutine
	// and the time budget of an earlier turn never settle a later one.
	resume bool
	turn   int
	// wave is the number of the delegate call that started the job (wave.go).
	wave int
}

// worktreeReporter is the optional contract a launched Process satisfies when
// it ran a worker in its own git worktree. The write route implements it so the
// queue can record where the worktree is and the commit it started from, which
// Diff reads later.
type worktreeReporter interface {
	worktreeInfo() (path, base string)
}

// sessionReporter is the optional contract a launched Process satisfies when
// its CLI wrote a session id that a later turn can resume. The queue reads it
// after the worker exits and stores only the opaque id on the job: it never
// learns which agent produced the id or how to resume it (the launcher owns
// that). A Process that does not implement it is simply not resumable.
type sessionReporter interface {
	SessionID() string
}

// presetSessionReporter is the optional contract a launched Process satisfies
// when its launcher knew the session id before the process started (a generated
// id passed to the CLI). The queue reads it at start and records the id on the
// job right away, so a worker cancelled on its very first turn, killed before
// its CLI can write any output, still has an id to delete its transcript with.
// A Process whose CLI assigns the id only at runtime reports "" here and is
// known only through sessionReporter after it exits. The queue stays agnostic:
// it reads an opaque id, never which agent produced it.
type presetSessionReporter interface {
	PresetSessionID() string
}

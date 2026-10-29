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
)

// Finished reports whether a state ends the job.
func (s State) Finished() bool { return s != Queued && s != Running }

// Spec is what one job asks for. Task is the prompt: it is held in memory for
// the life of the job and never written to disk (spec R6).
type Spec struct {
	Agent  string
	Model  string
	Effort string
	Dir    string
	Task   string
	Budget time.Duration
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
}

// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/workers"
)

// send gives an existing worker more work on the same conversation. It is the
// MCP surface over Queue.Continue (worker continuation, task T3): the queue owns
// the decision and the isolation, this file only maps the arguments in, the
// queue's errors out, and reports what happened. It is registered by WorkTools
// in work_tools.go; kept here so the delegate/collect/cancel/diff tools and the
// continuation tool each stay a readable file.

const sendSchema = `{"type":"object","properties":{"job_id":{"type":"string","minLength":1,"maxLength":16,"description":"The worker id delegate returned, the same id collect, cancel and diff take."},"task":{"type":"string","minLength":1,"maxLength":4000,"description":"The follow-up instruction for the worker, self-contained like a delegate task."}},"required":["job_id","task"],"additionalProperties":false}`

const sendDesc = "Gives an existing worker more work on the same conversation, so it keeps the context it already built. Use send for a follow-up that is related to what that worker just did: it reuses the worker's own dir, tier and effort, never climbs a tier, and never changes the mode. Use delegate instead for unrelated work, which has no shared context to reuse and starts a fresh worker. If the worker is still working, your task waits and runs as its next turn; if it has finished, the worker is resumed to run it as soon as a worker slot is free, under the same concurrency limit as delegate. The reply says whether the task was queued (behind a running turn or waiting for a slot) or resumed a finished worker, and its position in that worker's queue. Read the result through collect like any worker turn. send is not idempotent: each call adds one more turn. An unknown id, a worker that cannot take another turn (cancelled, failed, or captured no session to resume), an empty or oversized task, a full queue, a spent token budget and the session's follow-up turn cap are each refused with a clear message."

type sendArgs struct {
	JobID string `json:"job_id"`
	Task  string `json:"task"`
}

// send hands more work to an existing worker on the same conversation, through
// Queue.Continue: a live worker takes the turn in its inbox and runs it next; a
// settled, resumable worker is resumed now. The queue owns the decision and the
// isolation (same dir, tier, effort, mode; never a tier climb); this tool only
// maps the arguments in and the queue's errors out, and reports what happened.
//
// The reply reads the worker's state after the turn was accepted: a turn still
// waiting in the inbox is "queued" with its 1-based position in that inbox, and
// a turn that resumed a finished worker is "resumed" with position 0 (it runs
// now, it is not waiting). Reading the real post-state keeps the reply honest
// even when a live worker settles between the call and the read.
func (d Deps) send(ctx context.Context, raw json.RawMessage) (Result, error) {
	if d.Work == nil {
		return Result{}, errNoWork
	}
	var a sendArgs
	if err := decodeStrict(raw, &a); err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(a.JobID) == "" {
		return Result{}, errors.New("job_id must name a worker")
	}
	// The task's own guards (empty, oversized) belong to Continue, so send maps
	// one source of truth for them; it does not pre-check and risk a different
	// message. The schema already requires a non-empty string, so a blank task
	// is a whitespace-only one, which Continue refuses with ErrEmptyTask.
	if err := d.Work.Continue(a.JobID, a.Task); err != nil {
		return Result{}, sendErrorMessage(err)
	}
	state, position := "resumed", 0
	for _, v := range d.Work.Workers() {
		if v.ID == a.JobID {
			if v.Pending > 0 {
				state, position = "queued", v.Pending
			}
			break
		}
	}
	data := map[string]any{"job": a.JobID, "state": state, "position": position}
	return d.envelope(ctx, outcome{data: data, harnesses: workers.WorkerHarnesses()})
}

// sendErrorMessage turns each Continue error into a message the model can read
// and act on, without any task text. An unexpected error is passed through so a
// new failure is never hidden behind a stale mapping.
func sendErrorMessage(err error) error {
	switch {
	case errors.Is(err, workers.ErrUnknownJob):
		return errors.New("no worker with that id: check the id delegate returned")
	case errors.Is(err, workers.ErrNotResumable):
		return errors.New("that worker cannot take another turn: it was cancelled, failed, or captured no session to resume; start a new worker with delegate")
	case errors.Is(err, workers.ErrEmptyTask):
		return errors.New("the task needs a task: send some work for the worker to do")
	case errors.Is(err, workers.ErrTaskTooLong):
		return fmt.Errorf("the task is over the size limit of %d bytes: shorten it", workers.MaxTaskBytes)
	case errors.Is(err, workers.ErrInboxFull):
		return fmt.Errorf("the worker already holds the most queued turns it may (%d): wait for collect to drain some before sending more", workers.MaxInbox)
	}
	return err
}

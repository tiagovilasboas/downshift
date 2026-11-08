// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/workers"
)

// steer corrects a worker mid-turn without losing its conversation. It is the
// MCP surface over Queue.Steer (worker continuation, task T4): the queue owns
// the interrupt and the isolation, this file only maps the arguments in, the
// queue's errors out, and reports what happened. It is registered by WorkTools
// in work_tools.go; kept here so the delegate/collect/cancel/diff/send tools and
// the steer tool each stay a readable file.

const steerSchema = `{"type":"object","properties":{"job_id":{"type":"string","minLength":1,"maxLength":16,"description":"The worker id delegate returned, the same id collect, cancel, diff and send take."},"message":{"type":"string","minLength":1,"maxLength":4000,"description":"The correction for the worker, self-contained like a delegate task; it runs before any work already queued."}},"required":["job_id","message"],"additionalProperties":false}`

const steerDesc = "Corrects a worker that is in the middle of a turn, running your correction first instead of waiting for the turn to finish. Use steer when a worker is going the wrong way and you want it to change course now, keeping the context it already built; use send when you only want to add a related follow-up after the current work, and delegate for unrelated work. steer stops the live turn and resumes the same conversation with your correction as its first turn, ahead of anything send already queued. It reuses the worker's own dir, tier and effort, never climbs a tier, and never changes the mode. Writes the worker already made to its worktree stay; only work the stopped turn had not yet saved to the conversation is lost. If the worker is not on a live turn, the correction simply jumps to the front of its queue and runs first; a finished worker resumes with it as soon as a worker slot is free. Read the result through collect like any worker turn. steer is not idempotent. A worker that cannot be steered (cancelled, failed, or finished with no session to resume) is refused: start a new worker with delegate. An unknown id, an empty or oversized message, a full queue, a spent token budget and the session's follow-up turn cap are each refused with a clear message."

type steerArgs struct {
	JobID   string `json:"job_id"`
	Message string `json:"message"`
}

// steer applies a correction to a worker through Queue.Steer: a live turn is
// interrupted and the correction resumes the same conversation first; an idle
// worker takes the correction at the front of its queue. The queue owns the
// interrupt and the isolation (same dir, tier, effort, mode; never a tier
// climb); this tool only maps the arguments in and the queue's errors out, and
// reports what happened.
//
// The reply reads the worker's state after the correction was accepted: a
// correction still waiting (a queued or blocked worker) is "queued" with its
// 1-based position, and a correction that interrupted a live turn or resumed a
// settled worker is "steering" with position 0 (it runs now). Reading the real
// post-state keeps the reply honest even when the worker settles between the
// call and the read.
func (d Deps) steer(ctx context.Context, raw json.RawMessage) (Result, error) {
	if d.Work == nil {
		return Result{}, errNoWork
	}
	var a steerArgs
	if err := decodeStrict(raw, &a); err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(a.JobID) == "" {
		return Result{}, errors.New("job_id must name a worker")
	}
	// The message's own guards (empty, oversized) belong to Steer, so steer maps
	// one source of truth for them; it does not pre-check and risk a different
	// message. The schema already requires a non-empty string, so a blank message
	// is a whitespace-only one, which Steer refuses with ErrEmptyTask.
	out, err := d.Work.Steer(a.JobID, a.Message)
	if err != nil {
		return Result{}, steerErrorMessage(err)
	}
	// Steer returns what it did, not a racy post-state: interrupting a live turn
	// settles and resumes asynchronously, so a snapshot right after the call
	// could still show the old turn. "steering" means the correction runs now
	// (it interrupted a live turn or resumed a settled worker); "queued" means
	// the worker had no live turn, so the correction waits at the front of its
	// queue and runs first when the worker's turn settles.
	state, position := "steering", out.Position
	if !out.Steering {
		state = "queued"
	}
	data := map[string]any{"job": a.JobID, "state": state, "position": position}
	return d.envelope(ctx, outcome{data: data, harnesses: workers.WorkerHarnesses()})
}

// steerErrorMessage turns each Steer error into a message the model can read and
// act on, without any task text. An unexpected error is passed through so a new
// failure is never hidden behind a stale mapping.
func steerErrorMessage(err error) error {
	switch {
	case errors.Is(err, workers.ErrUnknownJob):
		return errors.New("no worker with that id: check the id delegate returned")
	case errors.Is(err, workers.ErrNotSteerable):
		return errors.New("that worker cannot be steered: it was cancelled, failed, or finished with no session to resume; start a new worker with delegate, or send a follow-up to a worker that is still resumable")
	case errors.Is(err, workers.ErrEmptyTask):
		return errors.New("the message needs text: send the correction for the worker to follow")
	case errors.Is(err, workers.ErrTaskTooLong):
		return fmt.Errorf("the message is over the size limit of %d bytes: shorten it", workers.MaxTaskBytes)
	case errors.Is(err, workers.ErrInboxFull):
		return fmt.Errorf("the worker already holds the most queued turns it may (%d): wait for collect to drain some before steering", workers.MaxInbox)
	}
	return err
}

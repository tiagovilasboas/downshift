// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

import "errors"

// ErrCapReached refuses a Delegate that would take the session past its
// maximum number of worker jobs (SetMaxJobs).
var ErrCapReached = errors.New("worker cap reached: this session already started its maximum number of workers; fix: raise orchestration.max_workers_per_session or start a new session")

// DefaultMaxJobs is the default cap on worker jobs started in one session.
const DefaultMaxJobs = 32

// ErrTurnCapReached refuses a send or steer that would take the session past its
// maximum number of follow-up turns.
var ErrTurnCapReached = errors.New("follow-up turn cap reached: this session already ran its maximum number of send and steer turns; fix: start a new session, or raise orchestration.max_workers_per_session (the turn cap is that many times TurnsPerWorker)")

// TurnsPerWorker sets the session's cap on follow-up turns (send and steer) as
// a multiple of the job cap: 4 x 32 = 128 by default, none when the job cap is
// off. With the token budget and the concurrency limit, it bounds how much a
// looping orchestrator can run through follow-ups on finished workers.
const TurnsPerWorker = 4

// turnCap is the most follow-up turns this queue accepts; 0 means no cap. The
// caller holds the lock.
func (q *Queue) turnCap() int { return q.maxJobs * TurnsPerWorker }

// SetMaxJobs sets the most jobs this queue will ever accept; 0 means no cap.
func (q *Queue) SetMaxJobs(n int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if n < 0 {
		n = 0
	}
	q.maxJobs = n
}

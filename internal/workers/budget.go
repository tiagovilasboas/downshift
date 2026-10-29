// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

import "fmt"

// TokenBudgetEnv names the per-CLI token budget the owner sets. It is read at
// the edge (cmd/downshift) and passed in with SetTokenBudget; zero or unset
// means no budget.
const TokenBudgetEnv = "DOWNSHIFT_WORKER_TOKEN_BUDGET"

// End is what the queue reports when a job settles: the agent (CLI), how it
// ended and the tokens it reported. It never holds the task text or a path.
type End struct {
	Agent   string
	Outcome State
	Tokens  *Tokens
}

// ErrTokenBudget refuses a delegation once an agent has spent its budget.
type ErrTokenBudget struct {
	Agent  string
	Used   int64
	Budget int64
}

func (e ErrTokenBudget) Error() string {
	return fmt.Sprintf("the %s workers have spent %d tokens of their %d-token budget (%s); no new %s worker starts in this session",
		e.Agent, e.Used, e.Budget, TokenBudgetEnv, e.Agent)
}

// spend is the queue's token ledger: tokens reported by finished workers,
// summed per agent for the life of the queue (one server session). A worker
// that is still running, or that was killed before it reported usage, is not
// in the sum. Its fields are touched only under the queue's lock.
type spend struct {
	used   map[string]int64
	budget int64
	onEnd  func(End)
}

// check refuses an agent whose recorded tokens reached the budget.
func (s *spend) check(agent string) error {
	if s.budget <= 0 || s.used[agent] < s.budget {
		return nil
	}
	return ErrTokenBudget{Agent: agent, Used: s.used[agent], Budget: s.budget}
}

// record adds a settled job's tokens to the ledger and reports the end. The
// observer runs on its own goroutine so a slow log write never holds the queue.
func (s *spend) record(agent string, outcome State, tokens *Tokens) {
	if tokens != nil {
		s.used[agent] += tokens.Total
	}
	if s.onEnd != nil {
		go s.onEnd(End{Agent: agent, Outcome: outcome, Tokens: tokens})
	}
}

// SetTokenBudget sets the per-agent token budget; zero or less turns it off.
func (q *Queue) SetTokenBudget(perAgent int64) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.spend.budget = perAgent
}

// OnEnd sets the observer told about every settled job (telemetry, at the edge).
func (q *Queue) OnEnd(f func(End)) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.spend.onEnd = f
}

// TokensUsed returns the tokens recorded for an agent so far.
func (q *Queue) TokensUsed(agent string) int64 {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.spend.used[agent]
}

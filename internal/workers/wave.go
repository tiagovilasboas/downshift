// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

import "fmt"

// A wave is the set of workers one delegate call starts. It may carry a token
// budget of its own, and the whole session (the orchestration) may carry a cap.
// Both count only tokens a finished worker reported: a worker still running, or
// one whose CLI wrote no usage, adds nothing, so an unknown count never blocks
// (doubt stays allow). A limit of zero or less is no limit.

// WaveLimits are the token limits one delegate call brings.
type WaveLimits struct {
	// Wave is the budget of this call's workers together.
	Wave int64
	// Orchestration is the cap on every token the session's workers spent; a
	// call is refused once the spend has reached it.
	Orchestration int64
}

// ErrWaveBudget refuses a follow-up turn for a worker whose wave spent its budget.
type ErrWaveBudget struct {
	Wave   int
	Spent  int64
	Budget int64
}

func (e ErrWaveBudget) Error() string {
	return fmt.Sprintf("wave %d has spent %d tokens of its %d-token budget (budget_tokens); no more turns for its workers: start a new wave with delegate", e.Wave, e.Spent, e.Budget)
}

// ErrOrchestrationBudget refuses a new wave once the session spent its cap.
type ErrOrchestrationBudget struct {
	Spent int64
	Cap   int64
}

func (e ErrOrchestrationBudget) Error() string {
	return fmt.Sprintf("the workers of this session have spent %d tokens of the %d-token orchestration budget (orchestration.budget_tokens); no new wave starts", e.Spent, e.Cap)
}

// OverBudget is the one decision: a known spend at or past a positive limit.
func OverBudget(spent, limit int64) bool { return limit > 0 && spent >= limit }

// wave is one delegate call's ledger.
type wave struct {
	budget int64
	spent  int64
}

// waves numbers the calls and keeps each one's ledger. Touched only under the
// queue's lock.
type waves struct {
	last  int
	byNum map[int]*wave
}

// spend adds a settled worker's tokens to its wave. Unknown tokens add nothing.
func (w *waves) spend(num int, tokens *Tokens) {
	if x, ok := w.byNum[num]; ok && tokens != nil {
		x.spent += tokens.Total
	}
}

// check refuses a turn for a worker whose wave is over its budget.
func (w *waves) check(num int) error {
	if x, ok := w.byNum[num]; ok && OverBudget(x.spent, x.budget) {
		return ErrWaveBudget{Wave: num, Spent: x.spent, Budget: x.budget}
	}
	return nil
}

// WaveView is the state of one wave that has a budget.
type WaveView struct {
	Wave   int   `json:"wave"`
	Budget int64 `json:"budget_tokens"`
	Spent  int64 `json:"spent_tokens"`
	Over   bool  `json:"over_budget"`
}

// Waves lists the waves that carry a budget, oldest first, with their spend
// and whether each is over it. A wave without a budget is not listed.
func (q *Queue) Waves() []WaveView {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.wavesLocked()
}

func (q *Queue) wavesLocked() []WaveView {
	out := []WaveView{}
	for n := 1; n <= q.waves.last; n++ {
		if x := q.waves.byNum[n]; x != nil && x.budget > 0 {
			out = append(out, WaveView{Wave: n, Budget: x.budget, Spent: x.spent, Over: OverBudget(x.spent, x.budget)})
		}
	}
	return out
}

// DelegateWave is Delegate with token limits. It queues one job per spec,
// starts what the concurrency limit allows and returns the job ids and the
// number of the wave they form. It refuses first when the session already spent
// its orchestration cap or an agent spent its budget.
func (q *Queue) DelegateWave(specs []Spec, lim WaveLimits) ([]string, int, error) {
	for _, s := range specs {
		if s.Budget <= 0 {
			return nil, 0, ErrNoBudget
		}
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, s := range specs {
		if err := q.spend.check(s.Agent); err != nil {
			return nil, 0, err
		}
	}
	if total := q.spend.total(); OverBudget(total, lim.Orchestration) {
		return nil, 0, ErrOrchestrationBudget{Spent: total, Cap: lim.Orchestration}
	}
	if q.maxJobs > 0 && q.nextID+len(specs) > q.maxJobs {
		return nil, 0, ErrCapReached
	}
	q.waves.last++
	num := q.waves.last
	q.waves.byNum[num] = &wave{budget: lim.Wave}
	ids := make([]string, 0, len(specs))
	for _, s := range specs {
		q.nextID++
		id := jobID(q.nextID)
		s.ID = id
		q.jobs[id] = &job{id: id, spec: s, state: Queued, wave: num}
		q.waiting = append(q.waiting, id)
		ids = append(ids, id)
	}
	q.pump()
	q.notify()
	return ids, num, nil
}

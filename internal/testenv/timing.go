// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package testenv

import (
	"os"
	"testing"
	"time"
)

// TimingEnv is the environment variable that turns wall-clock performance
// budgets into hard test failures. CI sets it to 1 (test, race and test-os
// jobs). Locally it is unset, so a loaded machine cannot fail a test that
// merely ran slowly: the measured duration is logged instead.
//
// Only PERFORMANCE budgets ("this call took longer than X") go through this
// file. Correctness assertions, and hang or deadlock guards (a process must
// not outlive a generous timeout), stay unconditional.
const TimingEnv = "DOWNSHIFT_TEST_TIMING"

// BudgetsEnforced reports whether performance budgets fail the test.
func BudgetsEnforced() bool { return os.Getenv(TimingEnv) == "1" }

// CheckBudget compares a measured duration with its performance limit. Over
// the limit it fails the test when budgets are enforced and only logs
// otherwise. what names the measured operation.
func CheckBudget(t *testing.T, what string, got, limit time.Duration) {
	t.Helper()
	if got <= limit {
		return
	}
	if BudgetsEnforced() {
		t.Errorf("%s took %v, budget %v", what, got, limit)
		return
	}
	t.Logf("%s took %v, over the %v budget (not enforced; set %s=1)", what, got, limit, TimingEnv)
}

// SkipIfOverBudget skips the test when a run that depends on a production
// deadline (a hook's own 2.5 s context, say) overran that deadline on a loaded
// machine, because its output was then degraded by design and says nothing
// about correctness. Enforced runs (CI) never skip: the overrun then fails the
// correctness assertions that follow, as before. Call it right after the run
// and before those assertions.
func SkipIfOverBudget(t *testing.T, what string, got, limit time.Duration) {
	t.Helper()
	if got > limit && !BudgetsEnforced() {
		t.Skipf("%s took %v, over its %v deadline; machine too loaded to judge (set %s=1 to enforce)", what, got, limit, TimingEnv)
	}
}

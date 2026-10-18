<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# T6: gate the outcome-eval pin in CI (owner decision needed)

**Problem.** `internal/adapt/outcome_eval_test.go` pins the classifier quality
floor (37/40) against `~/Github/downshift-labs/eval/outcomes`, which does not
exist in CI, so the gate is skipped there. The 38 to 37 move accepted on
2026-10-12 is watched only on the maintainer machine.

**Options (pick one, do not implement before the owner decides).**
1. Add a CI step that checks out `downshift-labs` read-only and runs the test.
2. Vendor a prompt-free summary (task id, expected tier per recorded run) into
   `internal/adapt/testdata/` and test against it.
3. Keep as is and document that the gate is local.

Recommendation: option 2 (no new CI secret, no private content).

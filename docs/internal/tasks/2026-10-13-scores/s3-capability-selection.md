<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# S3: choose the model by capability needed for the task (shadow first)

Today the target comes from the catalog tier of the session ids (T2). Add a
capability path that uses public scores for the session's own enabled models.

Logic (`internal/core`, harness-agnostic):
- `core.ScoreHook` as in the README contract; absent hook or missing score for any
  candidate => this path abstains (the catalog-tier path decides, as today).
- Required capability from the classified tier, relative to the session itself:
  `need(tier) = lo + (hi-lo) * f(tier)` with `lo`/`hi` the min/max score among the
  session's selectable ids and `f(small)=0`, `f(mid)=0.60`, `f(frontier)=0.95`
  (tunable constants in one place, documented). Pick the CHEAPEST selectable id
  (catalog price if known, else list order) whose score >= need; quota,
  explicit_only and exhausted rules unchanged; never pick below the catalog-tier
  floor chosen by T2 for risk/COMPLEX decisions (risk floor, COMPLEX stay on the
  catalog path).
- Shadow by default: compute `ScoreWould` id on the Decision and record it in
  telemetry (new reason-free field `score_would`, prompt-free) and in `stats`
  (how often it differs and the price delta). `DOWNSHIFT_SCORE_ROUTING=1` makes
  it live. Never raises above the current model on an unconfident decision (R8
  still applies to the final verdict).
- Tests at `core.Route`/`PlanSession`: sessions of claude-code, codex, cursor and
  antigravity ids with a fake hook; abstain on a missing score; relative scaling
  (same models, narrower list); risk/COMPLEX unaffected; shadow does not change
  output; live mode picks the cheapest passing id; `Check` invariant test
  (`final_decision_invariant_test.go`) still green with the flag on.

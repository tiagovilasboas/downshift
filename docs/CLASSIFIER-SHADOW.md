# Candidate classifier shadow evaluation

`harness-downshift` by Tiago de Carvalho Vilas Boas
https://github.com/tiagovilasboas/downshift

## What runs today

Production hooks still use `core.Route` and its existing deterministic,
Naive Bayes and optional semantic augmentation paths. Routing V2 provides a
separate feature-based softmax classifier, training and offline comparison.
It is not the default hook pipeline. The default semantic embedder is hashing;
real MiniLM embeddings require an explicitly configured external helper.

This change adds an opt-in **observation path** to the shared feedback recorder
used by Claude Code, Cursor, Codex, Antigravity and KiroCrew. It records a local
candidate's predictions beside the production **recommendation**, under the
same feedback ID. Nothing from the candidate reaches the hook response or
the production model matcher. A candidate error cannot change that response.

## Run the loop

Build a candidate from a training-only dataset or explicitly reviewed events:

```bash
downshift train dataset.json --output=candidate.json
# Alternative: downshift train --from-events --output=candidate.json
```

Do not train on evaluation-only fresh/heldout2/blind-vitrine splits. Keep a
separate future evaluation cohort; once results guide tuning, that cohort is
no longer blind. Do not evaluate training events as proof of generalization.

Set the variable in the harness hook's environment (an absolute path is useful
because hooks may run from different working directories):

```bash
export DOWNSHIFT_SHADOW_WEIGHTS=/absolute/path/candidate.json
```

Run tasks normally. After reviewing the actual execution, use the feedback ID
printed by the hook:

```bash
downshift feedback <id> success
# Only add a minimum-tier label after explicitly reviewing that requirement:
downshift feedback <id> retry --retry-tier=FRONTIER --required-tier=FRONTIER
downshift shadow-report
# Optional explicit input; output is always JSON:
downshift shadow-report --events=/absolute/path/loop-events.jsonl
```

`success`, `retry` and `failed` without `--required-tier` remain quality
feedback; they do not become supervised tier labels. A previous implementation
inferred the selected tier on success. Historical records cannot distinguish
that inference from an explicit review: re-review those labels before training.
Correct an old review by recording feedback again with the same ID.

Disable observation by unsetting `DOWNSHIFT_SHADOW_WEIGHTS`. Do not copy this
file over production weights to enable shadow mode. The existing weights
override belongs to Routing V2 and does not promote V2 onto production hooks.

## Interpret the report

Each model group identifies the exact candidate bytes with a SHA-256 digest.
Artifact paths and free-text version fields are not persisted. Groups contain
counts of observations, prediction errors, disagreements, upshifts/downshifts,
reviewed outcomes and explicit tier labels. Label-based counts compare:

- Production recommendation against the reviewed requirement.
- Raw classifier argmax against that requirement.
- Candidate recommendation after the existing V2 safety floor and confidence
  policy against that requirement.

Raw and policy `FRONTIER → SMALL` counts are separate, with the number of
reviewed Frontier labels as denominator. A safety correction must not conceal
a classifier's unsafe raw prediction. Margin is top probability minus runner-up;
it is not a calibrated probability of execution success.

A production failure plus a candidate upshift is an interesting disagreement,
not evidence that the candidate's chosen model would have passed. Shadow mode
never executes that model. Paired outcome experiments are still needed.
`selected_tier` is a recommendation, not a provider acknowledgement; control
groups and preserved explicit models can run another tier. Review actual runs.

Missing/invalid artifacts produce `load_error` observations. Invalid probability
vectors and predictor failures are counted without storing backend error text.
The opt-in softmax path reads at most 64 KiB from a regular local artifact and
does not execute helpers, call providers or access the network. Raw task text
is never stored; existing numeric features and reviewed metadata remain local.

## Next: a semantic Decision Model

`shadow.Predictor` accepts transient text plus numeric features. This experiment
boundary allows a semantic backend without pretending a transformer can consume
the existing `domain.Classifier` feature contract directly. The implemented
backend is a trained softmax candidate, not MiniLM/ModernBERT fine-tuning.

Before implementing or promoting a neural backend:

1. Curate consented text training data separately. Thirteen stored signals
   cannot reconstruct prompts or train a text encoder.
2. Freeze tokenizer, representation, model, labels and preprocessing; identify
   the complete bundle, not just ONNX weights.
3. Measure latency p50/p95, memory, calibration and unsafe under-routing on
   unseen workloads. Keep safety/policy independent of the learned model.
4. Evaluate ONNX runtime packaging: many Go bindings require a native library
   and CGO. A local model does not automatically preserve today's portable
   `CGO_ENABLED=0`, zero-dependency binary. Declare this tradeoff explicitly.
5. Give inference a hard resource/deadline boundary before connecting it to
   hooks. The current bounded-file softmax adapter is not a timeout wrapper
   for arbitrary future predictors.
6. Run paired outcome checks and manually review promotion with rollback.

Neither automatic retraining nor automatic activation is part of this change.

# Multilingual E5 (ONNX INT8) as a routing candidate: evaluation, 2026-10-17

Downshift by Tiago de Carvalho Vilas Boas · https://github.com/tiagovilasboas/downshift

**Superseded 2026-10-17:** E5 shadow, E5 fuse, and the external MiniLM command
were removed from the product. This note is historical. The decision below
was "do not enable"; removal followed.

**Decision (historical): do not enable `DOWNSHIFT_E5_FUSE`.** No gain was proven. The three plumbing defects found during the evaluation (§3) are
fixed; they no longer stand between a measured gain and an enable decision. The baseline is measured and reproducible; the E5 side could not be
measured here because the model weights are unreachable from this environment,
and the full run is scripted for a machine that can reach them.

Scope set by the owner: evaluate only `intfloat/multilingual-e5-small` as ONNX
INT8, no other model, no fine-tuning; compare with the current classifier on the
M3 set and the five legacy splits; measure accuracy, coverage and latency; keep
E5 opt-in in shadow mode unless the gates in the initiative doc §15.6 pass.

## 1. What was measured

### 1.1 Baseline, current classifier (fuse off, shadow off)

Binary built from clean `2c8f3f4`; `core.ClassifyWithSemantic`, the same entry
`downshift benchmark` and `tools/multilang` use; all `DOWNSHIFT_*` unset, so the
default in-process hash boost is on. M3 fixture sha256
`d4213e51…8df61bc`, equal to `run-manifest.json` and unchanged since freeze
commit `c94bcad`. Full tables, per-item rows (no prompts), confusion matrices and
the regenerating script: `downshift-labs/eval/multilang/results/baseline-2026-10-17.{json,md}`,
`downshift-labs/scripts/baseline_eval.py`.

| Set | n | Tier acc. | Confident coverage | Acc. on confident | Strict acc. | Unsafe raw / confident | Warm p50 / p95 ms |
|---|---:|---:|---:|---:|---:|---|---|
| M3 | 170 | 34.1% | 19.4% | 100% | 19.4% | 74 / 0 of 77 | 0.23 / 3.22 |
| tasks | 200 | 73.5% | 61.0% | 95.1% | 58.0% | 0 / 0 of 50 | 0.22 / 0.50 |
| holdout | 300 | 100% | 96.0% | 100% | 96.0% | 0 / 0 of 75 | 0.23 / 0.68 |
| fresh | 100 | 53.0% | 37.0% | 94.6% | 35.0% | 11 / 0 of 25 | 0.31 / 0.61 |
| heldout2 | 120 | 41.7% | 23.3% | 78.6% | 18.3% | 18 / 3 of 30 | 0.27 / 0.45 |
| blind-vitrine | 60 | 46.7% | 38.3% | 65.2% | 25.0% | 7 / 0 of 20 | 1.71 / 4.25 |

"Unsafe" is a risky or frontier-expected item whose raw tier is below the
expected one, shown as raw count and confident-only count over the safety set.
"Strict" counts an abstention as not correct even when the frontier model was
kept. The five legacy splits match the previous labs baseline item by item; on
M3, accuracy is unchanged at 34.1% and unsafe confident fell from 10 to 0 after
H14, with coverage down from 25.3% to 19.4%.

M3 by group: EN 55% accuracy at 55% coverage, PT-BR 55% at 30%, ES 25% at 5%,
FR, DE and JA 25 to 30% at **0% coverage**, the 30 mixed-language prompts 0% at
0% coverage, delegation briefs 75% at 75%. Every confident M3 item is correct.
The classifier abstains on almost everything outside EN and PT-BR; that is the
safe behaviour and it is the gap an encoder would have to fill without creating
unsafe confident routes.

Latency of the current path: warm p95 between 0.45 and 4.25 ms per prompt, worst
warm maximum 6.2 ms, cold first call 8.2 ms, against a 1500 ms hook budget.

### 1.2 E5 side: what could be measured here

Model access: every route to the weights is refused by this environment's
egress proxy (CONNECT 403): huggingface.co and its CDNs, hf-mirror.com,
modelscope.cn, registry.ollama.ai, jsdelivr, unpkg, archive.org, Kaggle, Zenodo,
the pytorch download host; PyPI and npm are reachable but hold no package that
vendors multilingual-e5-small; fastembed's Google Storage mirror serves only the
large variant (403 on the small name). Full route table:
`downshift-labs/eval/multilang/results/e5-env-2026-10-17.md`.

So the measurements below use the real Go fuse path and the real helpers with
(a) a synthetic ONNX graph of identical architecture (vocab 250,037, hidden 384,
12 layers, 12 heads, FFN 1,536; 117.5M parameters; FP32 470 MB, dynamic INT8
quantization to 118 MB in 20.7 s) whose compute and load cost are representative
and whose outputs are not, and (b) random or replayed vectors. They settle the
latency and the plumbing, not the accuracy.

Latency, 4 vCPU shared box:

| Path | p50 ms | p95 ms |
|---|---:|---:|
| Python start, no imports | 24.5 | 33.4 |
| + numpy + onnxruntime + tokenizers imports | 240 | 297 |
| Cold: fresh process, import + INT8 session + one run (seq 32) | **1027** | **1538** |
| Cold, same for FP32 | 2094 | 5103 |
| Warm: one process, INT8, seq 32, 100 runs | 27.6 | 70.1 |
| Warm: INT8, seq 32, single thread | 11.5 | 15.2 |
| Warm: INT8, seq 128 | 24.8 | 43.7 |

Not included because the files are unavailable: the `transformers` import and
the 17 MB XLM-R `tokenizer.json` load, both on the cold path.

Go side, built from the working tree, 17 `try` cases and 10 hook cases: every
fail-open code behaves as specified (`missing_cmd`, `untrusted_cmd` for `bash
-c`, `curl`, symlink, relative path and glued `-c`, `load_prototypes` for a
symlinked store, `embed_error` for non-JSON); a slow helper is killed at 1001 ms
and the hook returns within 1027 to 1041 ms; the kill switch and `FUSE=true` leave
routing unchanged; `ABSTAIN` clears confidence without changing the class; a
trusted helper can raise TRIVIAL to SIMPLE; `FUSE=1` plus `SHADOW=1` with a
byte-identical command embeds once. No prompt text in any event.

## 2. Gates (initiative doc §15.6, enable) and their status

| Gate | Status | Why |
|---|---|---|
| 1. M3 unsafe confident = 0 with fuse on and off | **Not evaluable here** (off: 0, measured) | Needs real E5 vectors |
| 2. Five legacy splits: no rise in unsafe, no drop in tier accuracy vs fuse off | **Not evaluable here**; the structural risk found (§3.2) is fixed | Needs real vectors; the fuse-off floor now makes a drop below fuse-off impossible by construction |
| 3. Kill switch identical to `main` | **Pass** | Measured on 17 + 10 cases |
| 4. Warm ONNX p95 embed + classify ≤ 500 ms; cold Python is BLOCK | **BLOCK as wired** | A warm session is 11 to 70 ms p95, inside the gate, but the hook spawns a fresh process per call and that cold path is ~1.0 s p50 / 1.5 s p95 before tokenizer and `transformers` load, so under the 1 s per-call timeout most hooks fail open (the M4 result repeating) |
| 5. Prototypes include `ABSTAIN` / `FOREIGN`; mover slice non-zero | **Not evaluable here** | `development.json` has only the four complexity labels; the reserved centroids need real vectors |
| 6. `FUSE=1` + `SHADOW=1` single embed, inside budget | **Pass** after the §3.3 fix (measured with byte-identical commands; whitespace variants now share the key) | Before the fix, commands differing by one space embed twice and the hook hit the 1531 ms ceiling |

Merge gates (default off, CI) are green on `main`: the kill switch test, the
fail-open tests, the H14 acceptance with the fuse mocked on, and the default-off
benchmark parity test all pass in the current suite.

## 3. Plumbing findings (fixed the same day, after the measurements)

All three are fixed on `main` after this document's measurements, with tests
at the shared layer (`internal/core/e5_fuse_floor_test.go`,
`e5shadow_test.go`, `semantic_test.go`). The measurements above were taken
before the fixes and are left as recorded.

1. **No `e5_fuse` telemetry object and no `E5_BOOST` reason code.** A raise by
   the fuse is logged as `SEMANTIC_BOOST`, so E5 lifts and abstentions cannot be
   counted from `events.jsonl`, which §15.6 gate 7 and the Phase D analysis both
   assume. `internal/core/e5_fuse.go`, `internal/telemetry`. **Fixed:** the
   fuse writes `E5_BOOST` / `E5_ABSTAIN` (never `SEMANTIC_BOOST` for its own
   lift) and a prompt-free `e5_fuse` object limited to `model_id`,
   `nearest_label`, `similarity`, `action`, `latency_ms`, `error_code`.
2. **Fuse on can end below fuse off.** When the helper succeeds, the MiniLM /
   hash `MaybeAugment` slot is skipped ("one semantic slot"), so a prompt the hash
   boost would have lifted can lose that lift. In the smoke run with random
   vectors through the real Go path, `tasks-0046` went COMPLEX (fuse off) to
   MEDIUM (fuse on): `tasks` 73.5% to 73.0%, unsafe raw 0 to 1, exactly the
   pattern §15.6 gate 2 forbids. The enable candidate must keep the fuse-off
   class as a floor. `internal/core/semantic_fuse.go`, `e5_fuse.go`. **Fixed:**
   `fuseFloor` runs the in-process hash boost (no second helper process) and
   keeps its class when it is higher; such a lift is credited to the hash boost
   (`SEMANTIC_BOOST`), not to E5.
3. **The embed cache key is the raw command string.** "At most one embed" for
   `FUSE` plus `SHADOW` holds only for byte-identical `DOWNSHIFT_E5_CMD` and
   `DOWNSHIFT_E5_SHADOW_CMD`. Normalise or document. **Fixed:** the cache key
   folds whitespace (`NormalizeCmd`), which is the same split the helper is
   executed with.

## 4. What this means for the candidate

- **Accuracy and coverage gain: unproven.** No E5 vector has been scored on M3
  or the legacy splits in either repository; the Phase D checkbox "compare
  `would_lift` with production" is still open, and this environment cannot close
  it. The opportunity is real on paper (FR, DE, JA and mixed prompts have zero
  coverage today), and so is the risk: those are the groups where every current
  confident route is correct because there are none.
- **Latency: the warm engine fits, the fresh-process helper does not.** The
  architecture-identical graph embeds in tens of milliseconds warm, well inside
  the 500 ms gate; the per-call Python process costs about a second cold on this
  box, which is the same failure mode that rejected M4. Enabling the fuse with
  the current `CmdEmbedder` would mostly fail open and occasionally spend the
  whole budget. A warm sidecar or an in-process engine is a §15.7 non-goal today
  and is the real prerequisite.
- **Safety: the floor in §3.2 is in place**, so real vectors can be measured for
  gate 2 without the plumbing itself producing the forbidden pattern.

## 5. How to finish the measurement

On a machine that can reach huggingface.co:

```bash
cd downshift-labs
scripts/e5_eval.sh --tag 2026-10-xx --reserved <abstain-foreign centroids.json>
```

It stages fetch, dynamic INT8 quantization, INT8 versus FP32 cosine parity on 50
development sentences, centroids from `tools/multilang/development.json` (plus
the reserved labels you supply), cold and warm latency, fuse off and on over M3
and the five splits through the real binary, and writes
`results/e5-int8-<tag>.{json,md}` in the baseline schema with the §15.6 gate
table filled. `--smoke` was run end to end here (5 m 40 s): the kill switch is
identical and fuse off reproduces `baseline-2026-10-17.json` exactly.

To read a real shadow log instead of a fixture, run the hooks with `DOWNSHIFT_E5_SHADOW=1` for a while and then `downshift shadow-report --text` (or `--telemetry=<events.jsonl>` for a copied log): the "E5 shadow" section gives the error-code histogram, the would-lift rate by regex label, the abstain share, the false-downgrade proxy on risk decisions and the p50/p95/max latency against the 500 ms gate, all from the prompt-free `e5_shadow` object ([stats-export.md](../reference/stats-export.md)).

## 6. Sources

- Baseline: `downshift-labs/eval/multilang/results/baseline-2026-10-17.{json,md}`, `scripts/baseline_eval.py`, `scripts/baseline_eval/harness.go`
- E5 environment, routes, latency, Go-side tables, constants, commands: `downshift-labs/eval/multilang/results/e5-env-2026-10-17.{json,md}`
- Runbook: `downshift-labs/scripts/e5_eval.sh`, `e5_eval.py`, `e5_replay_helper.py`, `e5_embed_tokenizers.py`
- Contract and gates: `docs/internal/tasks/2026-10-16-multilingual-semantic-intelligence/README.md` §14, §15
- Helper: `tools/e5-shadow/`
- Earlier M4 result (MiniLM path, rejected: no M3 gain, cold helper 6.5 s, one-shot 1103 ms): initiative doc §5

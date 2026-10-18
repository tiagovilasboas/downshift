<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Multilingual evaluation decision, 2026-10-16

Downshift by Tiago Vilas Boas — https://github.com/tiagovilasboas/downshift

M3: authored private corpus frozen before candidate execution in private labs commits `653bfd9` and `c94bcad` (final fixture). It contains 120 aligned EN/PT-BR/ES/FR/DE/JA prompts, 30 mixed-language prompts and 20 synthetic agent delegations. Labels are AI authored, with human adjudication pending. Existing five splits were neither modified nor used for training. The labels, prompts, vectors and detailed reports remain private.

M4: reject this candidate for runtime adoption. Cached `paraphrase-multilingual-MiniLM-L12-v2` was evaluated locally on CPU, using independent multilingual development centroids. M3 tier accuracy remains 34.1% and confident coverage remains 25.3% (baseline and candidate at M4 time). Relative to the authored labels, both had **10** confident recommendations below the labelled floor on risky/frontier tasks (classifier recommendations, **not** evidence of executed unsafe rewrites).

**Update (2026-10-16, `main` @ `18abffc`):** H14 regex/guard work drives **M3 unsafe confident to 0** without changing Route() semantics for legacy splits (`cls-after` stable locally). M4 conclusion unchanged: no runtime adoption of MiniLM centroids. Human adjudication of M3 labels is still pending for absolute tier accuracy.

| Classifier split | Baseline tier accuracy | Candidate tier accuracy | Confident coverage (both) |
|---|---:|---:|---:|
| tasks | 73.5% | 73.0% | 61.0% |
| holdout | 100.0% | 100.0% | 96.0% |
| fresh | 53.0% | 53.0% | 37.0% |
| heldout2 | 41.7% | 41.7% | 23.3% |
| blind-vitrine | 46.7% | 46.7% | 38.3% |

One additional raw below-floor frontier recommendation appears on `tasks`; no additional confident unsafe recommendation appears on any split. The candidate therefore fails the conservative safety criterion as well. This is a classifier-level comparison using `core.ClassifyWithSemantic`, not the full benchmark/router gate; a different metric must not be silently substituted for benchmark results.

Cold import/model load measured 6536ms; persistent Python encoding p95 measured 93.7ms on 170 items. The existing external helper timed out after 1103ms with no embedding, before the 1500ms total hook budget could be met. The warm measurement is a persistent Python process, **not `downshift serve` integration**. Full precision CPU only; quantization and warm service integration remain untested. Because the candidate already fails coverage/safety/cold-budget criteria, no new runtime integration or language support was shipped.

M5: pending external billing constraint. Native Claude auth reported a first-party Pro subscription; local account metadata reported extra usage enabled. The user authorized subscription inference with no additional charges. No prompt was sent and no API was invoked. Private labs now has a fixed-prompt, tool-free native CLI runner that refuses enabled/unknown extra usage, requires explicit subscription-only opt-in and stops on errors without fallback. Sonnet 5.5 and a smaller model remain unmeasured; accuracy, latency, tokens and cost per 1000 prompts are unknown. The CLI's API-only budget flag cannot prove subscription extra usage is disabled. Do not claim the reference ceiling is complete.

Guia inferencial / behaviour: frozen labels and independent development set; sensor computacional / behaviour: corpus validation, SHA-256, unsafe-downshift criteria and measured latency. Guia inferencial / architecture fitness: offline candidate and subscription-only boundary; sensor computacional / architecture fitness: helper timeout and native inference preflight. Runtime policy remains conservative until measured evidence clears these gates.

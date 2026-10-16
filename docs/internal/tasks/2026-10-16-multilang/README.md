<!-- Copyright (c) 2026 Tiago de Carvalho Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Multilingual classification, 2026-10-16

Source: language audit of 2026-10-16 (25 task rows in EN, PT-BR, ES, FR, DE, JA, plus 15 mixed-language prompts) and the
question of whether the regex classifier is too weak. Findings: a prompt entirely in ES/FR/DE/JA is fail-safe (nothing
matches, the requested model is kept); a prompt that mixes an English trivial verb with a risk written in another language
got a confident downshift to small (16 cases); Portuguese lacks several risk terms. Outside the 40 EN prompts of the outcome
suite the classifier is confident on 23% to 38% of tasks, so most decisions abstain.

Layering rule applies (AGENTS.md): behaviour in shared layers, tests at `core.Route`. The risk floors and guardrails stay the
safety net whatever classifier sits above them.

| ID | Severity | Task | Area |
|----|----------|------|------|
| M1 | High | Language guard: unsupported-language prompts keep the requested model; record it in telemetry | core classifier |
| M2 | High | Portuguese risk terms (permissão, privilégios, segurança, injeção, dados pessoais, CPF/CNPJ of a person) and PT prompt-injection phrases | core risk |
| M3 | High | Build a multilingual evaluation set (hand-written, mixed-language included), kept apart from the benchmark splits | eval |
| M4 | High | Candidate A: local multilingual embedding behind the existing helper path, compared with the current classifier | semantic |
| M5 | Medium | Candidate B: small LLM as an opt-in reference ceiling (Sonnet 5.5), never on the hot path by default | semantic |
| M6 | High | Why the router keeps Opus: bounded delegation prompts must be able to reach MEDIUM confidently | core classifier |
| M7 | Medium | Document supported languages and the abstention behaviour | docs |

Order: M1 and M2 first (safety, independent of the rest). M3 before M4 and M5. M6 runs in parallel and needs its own
held-out set from real delegation prompts. M7 after M4/M5 decide the supported set.

Rules for every task: no push without approval, header on new Go files, tests first for behaviour changes, no tuning on the
evaluation splits (`tasks`, `holdout`, `fresh`, `heldout2`, `blind-vitrine`), `go test -p 2 ./...` before done.

## Status (2026-10-16, end of session)

| ID | Status | Evidence |
|----|--------|----------|
| M1 | Done | `48c0dc4`, `def8ac3`; telemetry reason code `UNSUPPORTED_LANGUAGE` |
| M2 | Done | `48c0dc4`; the Portuguese medium verbs remain a separate change |
| M3 | Done, adjudication pending | labels frozen in labs (`653bfd9`), 170 items |
| M4 | Rejected | no gain over the current classifier on the frozen set (labs `b640e92`) |
| M5 | Blocked | no request sent; extra usage was enabled on the account |
| M6 | Partly done | H1 and H5 landed; see [M6 diagnostic evidence](../m6-delegation-evidence-2026-10-16.md) |
| M7 | Partly done | README and `complexity-weights.md` describe the guard; update after H14 |

Evaluation details and limitations: [evaluation-status.md](evaluation-status.md). Session-first model resolution (H13) keeps native metadata apart from catalog prices; discovery records availability only when the harness supplies evidence and does not invent capability, pricing or credit.

Open work is mapped in [2026-10-16-handoff](../2026-10-16-handoff/README.md).  
**Priority initiative (2026-10-16):** [Multilingual semantic intelligence](../2026-10-16-multilingual-semantic-intelligence/README.md) — diagnosis and Phase A–E contract; supersedes non-essential handoff until Phase B baseline exists.

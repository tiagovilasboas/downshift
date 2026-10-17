<!-- Copyright (c) 2026 Tiago de Carvalho Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# M1: language guard

**Problem.** A prompt with an English trivial verb ("rename", "format") and a risk written in ES/FR/DE/JA is classified
confidently as TRIVIAL and downshifted to small, because the risk table has no terms in those languages (16 hand-written
cases, for example a rename in the module that stores user passwords, written in Spanish).

**Do.** In `Classify`, mark the result unconfident and set `Classification.UnsupportedLanguage` when the prompt has 4 or more
letters of a non-Latin script, or at least 2 ES/FR/DE function-word markers (or letters such as ñ, ü, ß, è). The guard never
changes the class and never adds cost. Add it to telemetry as a reason code so its frequency in real use is known. Working
draft: `internal/core/lang_guard.go` (written in the audit, uncommitted when this task was filed).

**Done when.** The 16 unsafe prompts are held; English and Portuguese prompts with stray foreign bits (`del`, `λ`, `müllerId`)
stay confident; all five benchmark splits are identical item by item; reason code in `stats --detail` and `why`.

**Implemented, 2026-10-16.** The guard now propagates `UnsupportedLanguage` through the shared decision into the prompt-free
`UNSUPPORTED_LANGUAGE` reason code, shown by `stats --detail` and `session --why`. Coverage includes the full prompt beyond
the 32 KiB scoring cap and the no-signal default. Existing deterministic lifts and the full-input risk floor are unchanged.
Shared-runner acceptance tests exercise all five adapters; KiroCrew now exposes its requested model to the shared runner so
an allowed spawn records the known baseline instead of `unknown`.

**Evidence.** `go test ./internal/adapters/kirocrew ./internal/core ./internal/telemetry` passes. The QA acceptance suite covers
the five adapter paths and privacy assertions. Guia: language coverage and abstention contract (inferencial, behaviour).
Sensor: shared guard, regression tests and reason-code telemetry (computacional, behaviour / architecture fitness).

Attribution: Downshift by Tiago de Carvalho Vilas Boas — https://github.com/tiagovilasboas/downshift

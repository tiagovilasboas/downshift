<!-- Copyright (c) 2026 Tiago de Carvalho Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# M6 delegation evaluation — rejected candidate

Local diagnostic artifacts were read from the existing isolated `opus-data` evaluation. Private delegation text and
transcripts are excluded from this repository. These figures describe an offline candidate, not a shipped change.

| Temporal split | Decisions | COMPLEX before → candidate | Unconfident before → candidate | Allowed downshift before → candidate |
|---|---:|---:|---:|---:|
| Development, Oct 6–14 | 117 | 64 → 26 | 75 → 36 | 16 → 50 |
| Held out, Oct 14–16 | 79 | 25 → 13 | 49 → 37 | 16 → 22 |

Risk category counts stayed 35 and 38, respectively. All 89 original COMPLEX decisions had the length nudge; 49 were
unconfident. This shows length and brief scaffolding contribute to the problem, but does not independently label which
real tasks can safely run at mid tier.

The `tasks` (200), `holdout` (300), `fresh` (100), `heldout2` (120) and `blind-vitrine` (60) splits produced identical
before/after classifier records: 780 of 780 rows, including scores, confidence, risk floor and safe verdict.

AppSec tested four synthetic long briefs with scope, verification and answer-budget constraints against both classifiers:

| Genuine task | Candidate result | Safety outcome |
|---|---|---|
| Investigate a memory leak | COMPLEX → confident MEDIUM | New frontier → mid downshift |
| Implement a recursive-descent parser | COMPLEX → confident MEDIUM | New frontier → mid downshift |
| Investigate a deadlock | COMPLEX → MEDIUM | Existing risk guard held the downshift |
| Work on a consensus protocol | Remained COMPLEX | Two matching COMPLEX rows preserved the class |

The candidate's rule drops a single COMPLEX signal when a brief appears bounded. Scope and answer limits do not prove
the underlying task is easy. Two newly allowed unsafe downshifts out of four adversarial cases reject this candidate;
the isolated experiment was not copied into production.

Guia: task constraints do not cancel hard-work signals (inferencial, behaviour).
Sensor: adversarial classifier and safe-verdict comparison alongside temporal and benchmark checks (computacional,
behaviour). This closes the evaluation decision, while a safe reduction in Opus usage remains future work.

Attribution: Downshift by Tiago de Carvalho Vilas Boas — https://github.com/tiagovilasboas/downshift

<!-- Copyright (c) 2026 Tiago de Carvalho Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# M7: document supported languages

**Problem.** The README says Portuguese is supported and is silent about other languages and about abstention.

**Do.** State in `README.md` and `docs/reference/complexity-weights.md`: English and Portuguese are classified by rules; a
prompt in another language keeps the requested model; the guard of M1 and the decision of M4/M5 once taken. Add the
reason code to `docs/reference/stats-export.md`.

**Done when.** The docs match the shipped behaviour and `internal/docscheck` passes.

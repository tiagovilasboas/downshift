<!-- Copyright (c) 2026 Tiago de Carvalho Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# T2: choose the target by catalog tier, not list position

**Problem.** `leastSessionModelAtOrAbove` maps tiers to list indexes
(small = first, mid = `len/2`, frontier = last). Measured with a confident MEDIUM
task on an opus parent: list `[haiku, sonnet, opus, fable]` stays on opus (mid
index is opus); `[haiku, opus]` stays on opus; `[sonnet, opus]` stays on opus
although sonnet is mid in the catalog. Adding Fable (as the README says to
opt in) breaks all downshift of mid tasks.

**Do.** When every id of the session list resolves in the resolver
(`res.LookupByID`), select the first id (in list order, least to most capable)
whose catalog tier is >= the needed tier, skipping blocked/exhausted/explicit_only
as today. Keep the index mapping only as the fallback for lists whose ids have
no catalog tier (operator ids unknown to the catalog). Keep the quota-after-floor
rule: an exhausted id must not lower the floor. Do not use names or prices.

**Done when.**
- Table-driven tests in `internal/core` for: 4-model list with fable, 2-model
  lists, sonnet-only, all-unknown ids (fallback), exhausted mid, explicit_only,
  upshift (`strongestSessionModel` unchanged unless the owner decides; leave a
  TODO note, do not change it).
- `docs/reference/session-models.md` "How a target is chosen" updated.
- Existing tests green; if a pin must change, stop and report.

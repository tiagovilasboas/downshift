<!-- Copyright (c) 2026 Tiago de Carvalho Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# T3: `stats` must be able to show a loss

**Problem.** `Event.RealSavedUSD()` clamps negative savings to zero and the
normalised metric credits only applied downshifts (upshift counts as 1.0), so a
router that overspends reports neutral. This is why the semantic-boost overspend
stayed invisible.

**Do.**
- Add `RealDeltaUSD()` (baseline minus actual, may be negative) and keep
  `RealSavedUSD()` for callers that need the clamp.
- In `Aggregate`, add `RealCostDeltaUSD` (signed sum) and `RealOverspendUSD`
  (sum of negative deltas, positive number), print them in `PrintStats` next to
  "Real saved", and add them to the export (`stats-export.md`, additive fields,
  older readers unaffected).
- Normalised metric: an applied UPSHIFT with a known price ratio adds its extra
  cost to `NormRouted`; unknown ratio stays neutral. Keep the existing lines.

**Done when.** Unit tests for negative delta, clamp preserved for the old
accessor, applied upshift increasing the routed units, export JSON round-trip
with the new fields, `docs/stats-export.md` updated, suite green.

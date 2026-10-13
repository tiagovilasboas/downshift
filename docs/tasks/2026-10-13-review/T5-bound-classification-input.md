<!-- Copyright (c) 2026 Tiago de Carvalho Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# T5: bound classification input

**Problem.** Classification cost is linear in prompt size times about 100 regex:
900 KB takes 1.7 s locally and about 5 s on CI. The hook reads up to 1 MiB.

**Do.** In `Classify`, score at most the first 32 KB (cut on a rune boundary) and
keep word-count rules working from the full text length. Add a test that a signal
at byte 40 000 is ignored by design and one at byte 100 is found; add a latency
test with a 900 KB prompt under 500 ms (loose bound); keep the 1 MiB hook limit.
Mention the cap in `docs/complexity-weights.md`.

**Done when.** Tests green, `TestClassifyLatencyLargePrompts` still passes, no
behaviour change for prompts under 32 KB (existing corpus tests unchanged).

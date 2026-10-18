<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# M5: candidate B, small LLM as a reference ceiling

**Problem.** We do not know how far any classifier can get on this task; without a ceiling the comparison has no scale.

**Do.** Classify the M3 set with Sonnet 5.5 (and, for cost, one smaller model) using a fixed prompt, outside the hook. This is
a measurement, not a product feature. If it is worth shipping, it is an opt-in mode with its own latency, cost and privacy
notes (the prompt leaves the machine), never the default.

**Done when.** The ceiling accuracy and cost per 1000 prompts are reported next to M4, and the opt-in design is either
written up or dropped.

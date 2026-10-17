<!-- Copyright (c) 2026 Tiago de Carvalho Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# M6: why the router keeps Opus

**Problem.** In real use the router rarely moves delegation off Opus: in one session 40% of Claude Code decisions are COMPLEX,
and bounded MEDIUM downshifts are held as unconfident. The likely cause is the structure of agent-written delegation prompts
(long, many paths), not the language.

**Do.** Extract real delegation prompts from local Claude Code transcripts, split by date (older for development, newer held
out), attribute each COMPLEX or unconfident result to the rule that caused it, judge on a sample which tier was adequate, and
propose the smallest change that lets clearly bounded prompts reach MEDIUM confidently. The risk floor must not weaken.

**Done when.** The held-out set and every benchmark split show no increase of frontier tasks sent to mid and no risky prompt
losing its floor; or the change is rejected with the numbers.

**Decision, 2026-10-16: candidate rejected.** Reused the existing concurrent diagnostic in an isolated evaluation checkout;
the production scorer was not changed. See [evaluation evidence](../m6-delegation-evidence-2026-10-16.md).

Across 196 real delegations split chronologically (117 development, 79 held out), all 89 COMPLEX classifications carried
the long-prompt nudge and 49 were unconfident. The candidate treated bounded delegation briefs as MEDIUM evidence and
discounted an isolated COMPLEX keyword. Development COMPLEX results fell 64→26; held-out results fell 25→13. The five
existing benchmark splits (780 rows) stayed identical item by item.

However, four independent synthetic adversarial briefs exposed three COMPLEX→MEDIUM changes. Two caused actual newly
allowed frontier→mid downshifts (memory-leak investigation and recursive-descent parser); one was held by the existing risk
guard. The candidate therefore fails the acceptance criterion despite unchanged benchmarks. No risk floor was removed
in these four cases. Future work needs bounded-brief adversarial coverage before another scorer change.

Guia: preserve genuine complexity and risk in bounded delegations (inferencial, behaviour).
Sensor: temporal evaluation, five benchmark splits and adversarial downshift checks (computacional, behaviour).

Attribution: Downshift by Tiago de Carvalho Vilas Boas — https://github.com/tiagovilasboas/downshift

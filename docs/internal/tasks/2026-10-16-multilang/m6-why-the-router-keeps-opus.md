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

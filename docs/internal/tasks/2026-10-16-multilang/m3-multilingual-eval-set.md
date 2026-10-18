<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# M3: multilingual evaluation set

**Problem.** Every classifier decision so far was measured on English-dominant sets. There is no data to compare candidates
for other languages, and the five existing splits must not be used to tune.

**Do.** Write a new set (private, in downshift-labs, not published): about 120 prompts, each with an expected tier and a
risk flag, in EN, PT-BR, ES, FR, DE, JA, 20 per language with the same meaning per row, plus about 30 mixed-language prompts
(English verb plus risk in another language) and 20 realistic long delegation prompts written by an agent. Label before
running any candidate. Record who labelled and when.

**Done when.** The set exists with labels fixed in a commit before M4 and M5 run. Metrics defined up front: tier accuracy,
coverage (confident decisions), unsafe downshifts (risky or frontier task sent below its tier, the rejection criterion), p95
latency.

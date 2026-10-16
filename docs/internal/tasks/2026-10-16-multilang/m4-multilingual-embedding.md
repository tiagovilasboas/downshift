<!-- Copyright (c) 2026 Tiago de Carvalho Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# M4: candidate A, local multilingual embedding

**Problem.** The semantic layer is a hash embedder built from mostly English prompts; the optional model is English-only.

**Do.** Evaluate a small multilingual sentence embedding (for example paraphrase-multilingual-MiniLM-L12-v2, quantized) through
the existing external-helper path (`DOWNSHIFT_MINILM`), with centroids or nearest neighbours built from labelled prompts in
several languages. It may only raise a class, as today. Measure on the M3 set: accuracy, coverage, unsafe downshifts,
latency with a cold helper and with `downshift serve` keeping the model loaded. The hook budget is about 1.5 s and the hot
path has no network.

**Done when.** A report compares the current classifier and this candidate on M3 and on the five splits (no tuning on them).
Adopt only if coverage rises materially with no increase in unsafe downshifts and the latency fits the budget.

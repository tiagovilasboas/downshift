# Hash embedder audit — 2026-10-17

Downshift by Tiago de Carvalho Vilas Boas
https://github.com/tiagovilasboas/downshift

Read-only inventory of the in-process word-hash embedder at `2b0cfcd`.
This note does not change the algorithm, does not define a second embedder,
and does not report accuracy, coverage, or cost. `BuildHashPrototypes` labels
its store `hash-embed-v1` (`internal/semantic/prototypes_gen.go:37`). Nothing
below is an implementation of the papers it cites.

This repository does not ship benchmark datasets
(`benchmark/README.md:3`). No figure in this note was measured on a dataset.

## What the code does

### `HashEmbed` — unigram vector, then L2

`internal/semantic/hashembed.go:12` sets `defaultEmbedDim = 384`.
`HashEmbed` (`hashembed.go:16-50`) uses that size when `dim <= 0`
(`hashembed.go:17-18`).

The prompt is lowercased and split on whitespace with `strings.Fields`
(`hashembed.go:21`). Each field is one unigram. An empty split returns the
zero vector (`hashembed.go:22-24`).

Each word is hashed with FNV-64a (`fnv.New64a`, `hashembed.go:26-28`). The
inner loop then writes **every** coordinate (`hashembed.go:29-37`):

```go
bit := (sum >> (i % 56)) & 1
if bit == 1 {
    vec[i] += 1
} else {
    vec[i] -= 0.5
}
```

The shift is `i % 56`, so the same 56 bits of the 64-bit hash are reused
across the 384 coordinates. After every word has been added, the vector is
divided by its Euclidean norm when that norm is nonzero (`hashembed.go:39-49`).
`TestHashEmbed_UnitNorm` checks that property
(`internal/semantic/semantic_test.go:99-107`).

`HashEmbedder.Embed` (`hashembed.go:58-63`) calls `HashEmbed`, using 384 when
`Dim` is 0.

### `MaybeAugment` — fixed 0.45, raise only

`MinSimilarity` is the constant `0.45` (`internal/semantic/augment.go:39`).

`MaybeAugment` (`augment.go:45-58`) returns the incoming label when the boost
is off, when embedding fails, or when the prototype store does not load.
`enabled` is false only for `DOWNSHIFT_MINILM` in `0`, `false`, `off`, or
`no` (`internal/semantic/embed.go:17-24`). Otherwise the boost is on.

`augmentVector` (`augment.go:75-88`) asks `Nearest` for one label and one
cosine. If that cosine is below `0.45`, it keeps the incoming label
(`augment.go:77-79`). If the semantic rank is less than or equal to the
incoming rank, it keeps the incoming label (`augment.go:82-84`). If the
regex result is already confident, it keeps the incoming label
(`augment.go:85-87`). Only a strictly higher semantic rank, above the
threshold, with regex confidence false, replaces the label (`augment.go:88`).
That is raise-only.

### `Nearest` — best cosine only

`PrototypeStore.Nearest` (`internal/semantic/types.go:34-48`) computes
`cosine` (`internal/semantic/math.go:8-21`) against each centroid and keeps
the single maximum. It returns that label and that score. It does not store
a second-best score, and it does not subtract one score from another. There
is no top-2 margin.

### `BuildHashPrototypes` — mean of hash vectors

`BuildHashPrototypes` (`internal/semantic/prototypes_gen.go:14-41`) embeds
each prompt with `HashEmbed`, sums those vectors per label, and divides by
the number of prompts (`prototypes_gen.go:23-33`). Labels are stored
uppercased (`prototypes_gen.go:34`). The centroid is that mean. This function
does not L2-normalise the mean again. Dimension defaults to 384
(`prototypes_gen.go:15-17`). The `Model` string is the `hash-embed-v1` line
cited above.

## Papers this code does not implement

The four papers below are a map for readers. Sharing a word with the code
(hash, n-gram, threshold, abstain) is not an implementation.

### Weinberger et al. 2009 — feature hashing

Kilian Weinberger, Anirban Dasgupta, John Langford, Alex Smola, and Josh
Attenberg, “Feature Hashing for Large Scale Multitask Learning,” ICML 2009.
Definition 1 hashes each feature index `j` to **one** bucket `h(j)` and one
sign `ξ(j) ∈ {+1, −1}`:

`φ_i(x) = Σ_{j : h(j) = i} ξ(j) x_j`

A feature updates a single signed coordinate. Collisions add inside that
bucket. The hashed vector stays sparse.

`HashEmbed` does the opposite. Every word updates every coordinate, the two
increments are `+1` and `−0.5` rather than a ±1 sign, and the result is then
L2-normalised. **This is not Weinberger hashing.**

### Joulin et al. 2017 — bag of tricks (n-grams)

Armand Joulin, Edouard Grave, Piotr Bojanowski, and Tomas Mikolov, “Bag of
Tricks for Efficient Text Classification,” EACL 2017, §2.2. They add a bag of
word n-grams so local order survives a bag-of-words model, and they bound
that dictionary with the Weinberger hashing trick. Their sentiment table
reports a gain from adding bigrams.

`HashEmbed` never builds a bigram, trigram, or character n-gram. Adjacent
words only meet as separate unigram updates (`hashembed.go:21` and
`hashembed.go:25`). **There are no bigrams.**

### Gangrade et al. 2021 — one-sided selective classification

Aditya Gangrade, Anil Kag, and Venkatesh Saligrama, “Selective Classification
via One-Sided Prediction,” AISTATS 2021. Selective classification lets the
model abstain. Their method fits class-wise one-sided empirical risks and
keeps, per class, a decision set with few false positives. Prediction is the
accepted set; the rest is abstention.

`Nearest` returns only the best cosine. `augmentVector` then applies the
fixed cutoff and the rank check above. No second-best score is computed, so
**there is no top-2 margin**, and there is no class-wise one-sided risk or
learned decision set. The raise-only rule is not Gangrade’s selector.

### Fisch et al. 2022 — calibrated selective classification

Adam Fisch, Tommi Jaakkola, and Regina Barzilay, “Calibrated Selective
Classification,” TMLR 2022. They train a selector so that the predictions
the model still makes have calibrated uncertainty (their “selective
calibration”), including under a coverage constraint and a robust training
scheme.

`MinSimilarity` is the literal `0.45` in source (`augment.go:39`). It is not
fit on labels, not a quantile of a calibration set, and not a learned
selector. **The 0.45 gate is a fixed threshold, not calibrated selective
classification.**

## Absent, in one place

1. Weinberger feature hashing: one signed bucket per feature. The code writes
   every coordinate of every word.
2. Joulin word n-grams: no bigrams.
3. A top-2 margin: `Nearest` keeps the best cosine only.

The 0.45 gate stays a fixed threshold. It is not Gangrade’s one-sided
selector and not Fisch’s calibrated selector.

## Sources

- Code at `2b0cfcd`: `internal/semantic/hashembed.go`, `augment.go`,
  `types.go`, `math.go`, `embed.go`, `prototypes_gen.go`,
  `semantic_test.go`; `benchmark/README.md`.
- Weinberger, Dasgupta, Langford, Smola, Attenberg. Feature Hashing for
  Large Scale Multitask Learning. ICML 2009.
  https://doi.org/10.1145/1553374.1553516
- Joulin, Grave, Bojanowski, Mikolov. Bag of Tricks for Efficient Text
  Classification. EACL 2017. https://aclanthology.org/E17-2068/
- Gangrade, Kag, Saligrama. Selective Classification via One-Sided
  Prediction. AISTATS 2021. https://proceedings.mlr.press/v130/gangrade21a.html
- Fisch, Jaakkola, Barzilay. Calibrated Selective Classification. TMLR 2022.
  https://arxiv.org/abs/2208.12084

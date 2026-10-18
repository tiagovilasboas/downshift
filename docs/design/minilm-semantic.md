# Semantic boost — in-process word hash

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

The optional semantic layer raises complexity when regex confidence is low. It
runs **inside the Go binary**: no Python, no ONNX, no subprocess on the hook path.

## Algorithm

1. Tokenise the prompt (lowercase words).
2. For each word, update a 384-dimensional vector using **FNV-1a 64-bit** (see
   `internal/semantic/hashembed.go`).
3. L2-normalise the vector.
4. Nearest centroid in `internal/semantic/data/prototypes.json` (cosine similarity).
5. If similarity ≥ `MinSimilarity` (0.45), the semantic label ranks above the regex
   label, and the regex result was not confident, raise the label (monotonic).
6. Never downgrade; never lift a **no-signal** Medium default; never create
   **COMPLEX** unless regex already scored Complex.

## Operator knob

```bash
# Default: boost on
export DOWNSHIFT_MINILM=0   # opt out entirely
```

`downshift config` shows `semantic_boost` as `on (hash embedder)` when enabled.

## Telemetry

A lift clears classifier confidence and adds reason code `SEMANTIC_BOOST`.

## Retraining centroids

Maintainers can rebuild hash prototypes from labelled fixtures (see
`semantic.RefreshHashPrototypesFromBenchmark` and `benchmark/README.md`). That
does not change runtime behaviour until `prototypes.json` is updated in the repo.

## Removed (2026-10-17)

External MiniLM embed commands (`DOWNSHIFT_MINILM_EMBED`), multilingual E5
shadow/fuse, and Graphify escalation were removed to keep Downshift focused on
deterministic token optimisation for spawned subagents.

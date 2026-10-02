# MiniLM semantic boost (P4.9)

Local embedding layer beside the regex classifier — on by default, no LLM.
Same role as a small decision model (Jev/Laya): finite labels only.

## Paths

| Path | Role |
|------|------|
| `internal/semantic/` | Prototypes, hash/MiniLM embedder, `MaybeAugment` |
| `internal/semantic/data/prototypes.json` | Embedded centroids (refresh below) |
| `internal/core/semantic_fuse.go` | Wires boost into `Route` / `ClassifyWithSemantic` |
| `tools/minilm/README.md` | Train real MiniLM centroids + embed script |

## Default

Semantic boost is **on**. The shipped hash embedder runs in-process (no Python, no network).

```bash
# turn off
export DOWNSHIFT_MINILM=0

# optional neural MiniLM; hash is used if this command fails
export DOWNSHIFT_MINILM_EMBED="python3 tools/minilm/embed_stdin.py"
```

Rules:

- **Local first** — hash prototypes are embedded in the binary.
- **Fallback** — external embed failure returns to hash; hash or store failure leaves the regex label.
- **Monotonic** — semantic never downgrades; only raises when regex is unconfident.
- **No prompts in telemetry** — unchanged privacy contract.

## Refresh prototypes

Hash (zero deps, CI-friendly):

```bash
go run ./tools/minilm/refresh_hash_prototypes.go
```

Neural centroids (not the default embedder; used only when `DOWNSHIFT_MINILM_EMBED` is a command):

```bash
python3 tools/minilm/train_prototypes.py
python3 tools/minilm/eval_holdout.py
```

`train_prototypes.py` writes `internal/semantic/data/minilm.json`. It does not replace the hash file `prototypes.json`.

Holdout measurement (2026-10-01, centroids fit on `benchmark/tasks.json` only):

| Classifier | Holdout tier accuracy | FRONTIER→MID |
|---|---|---|
| Regex (`DOWNSHIFT_MINILM=0`) | 98.0% | 8% (75 frontier tasks) |
| all-MiniLM-L6-v2 nearest centroid | 96.3% | 0% |

The neural model did not raise tier accuracy on this split. It removed the remaining frontier-to-mid misses. The shipped default stays the local hash embedder. Numbers are in `benchmark/minilm-holdout.json`.

## Tests

```bash
go test ./internal/semantic ./internal/core ./internal/benchmark -run MiniLM -v
go test ./internal/benchmark -run TestSeedDataset_MiniLM -v
```

`TestSeedDataset_MiniLM_ImprovesTierOrSafety` locks regression: with the hash boost on,
tier accuracy must not drop vs regex-only and FRONTIER→MID must not worsen.

## Beta tasks

- **P4.9** — shipped, on by default, hash embedder
- **P4.10** — measured; neural centroids are optional and do not replace the default

See [BETA-EXIT.md](BETA-EXIT.md).

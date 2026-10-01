# MiniLM semantic boost (P4.9)

Optional **local embedding** layer beside the regex classifier — same role as a
small decision model (Jev/Laya): finite labels, no LLM in the routing path.

## Paths

| Path | Role |
|------|------|
| `internal/semantic/` | Prototypes, hash/MiniLM embedder, `MaybeAugment` |
| `internal/semantic/data/prototypes.json` | Embedded centroids (refresh below) |
| `internal/core/semantic_fuse.go` | Wires boost into `Route` / `ClassifyWithSemantic` |
| `tools/minilm/README.md` | Train real MiniLM centroids + embed script |

## Enable

```bash
export DOWNSHIFT_MINILM=1
# Default embedder: deterministic hash (shipped prototypes)
export DOWNSHIFT_MINILM_EMBED=hash

# Or real MiniLM (Python):
# export DOWNSHIFT_MINILM_EMBED="python3 tools/minilm/embed_stdin.py"
```

Rules:

- **Fail-open** — if embed fails, regex result stands.
- **Monotonic** — semantic never downgrades; only raises when regex is unconfident or lower rank.
- **No prompts in telemetry** — unchanged privacy contract.

## Refresh prototypes

Hash (zero deps, CI-friendly):

```bash
go run ./tools/minilm/refresh_hash_prototypes.go
```

Real MiniLM (better quality, needs `sentence-transformers`):

```bash
python3 tools/minilm/train_prototypes.py
```

## Beta tasks

- **P4.9** — shipped (opt-in env, hash prototypes)
- **P4.10** — retrain with MiniLM + report tier-accuracy delta on holdout

See [BETA-EXIT.md](BETA-EXIT.md).

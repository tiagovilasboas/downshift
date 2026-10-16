# MiniLM semantic boost (P4.9)

Local embedding layer beside the regex classifier — on by default, no LLM.
Same role as a small decision model (Jev/Laya): finite labels only.

## Paths (public OSS)

| Path | Role |
|------|------|
| `internal/semantic/` | Prototypes, hash/MiniLM embedder, `MaybeAugment` |
| `internal/semantic/data/prototypes.json` | Embedded hash centroids (shipped in binary) |
| `internal/core/semantic_fuse.go` | Wires boost into `Route` / `ClassifyWithSemantic` |

## Default runtime

Semantic boost is **on**. The shipped **hash** embedder runs in-process (no Python, no network).

```bash
export DOWNSHIFT_MINILM=0          # disable boost
export DOWNSHIFT_MINILM_EMBED="…"  # optional external embed command; hash fallback on failure
```

Rules:

- **Local first** — hash prototypes are embedded in the binary.
- **Fallback** — external embed failure returns to hash.
- **Monotonic** — semantic never downgrades; only raises when regex is unconfident.
- **Needs regex evidence** — a prompt with no regex signal (`NoSignal`) is never lifted, and the boost can never produce COMPLEX on its own: Complex needs a regex signal, the risk floor or graph escalation. The hash centroids are nearly parallel, so its nearest class on a weak-signal prompt is close to noise.
- **Visible** — a lift sets `Decision.SemanticLift` and the reason code `SEMANTIC_BOOST`; `downshift stats` shows its share.
- **No prompts in telemetry** — unchanged privacy contract.

## Maintainer training (private)

Refreshing hash prototypes, training neural centroids, and holdout eval scripts
live in **downshift-labs** (`tools/minilm/`). Published neural vs hash numbers:
[benchmark/report.md](../../benchmark/report.md).

Do not commit maintainer benchmark JSON to this repo; see
[benchmark/eval-private.md](../../benchmark/eval-private.md).

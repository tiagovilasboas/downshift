# MiniLM semantic boost

On by default inside the `downshift` binary: local hash centroids, no network.
Optional **all-MiniLM-L6-v2** replaces the hash embedder when the command works;
if it fails, hash is the fallback. Opt out with `DOWNSHIFT_MINILM=0`.

## Train / refresh prototypes

```bash
pip install sentence-transformers
python3 tools/minilm/train_prototypes.py
```

Commits target: `internal/semantic/data/prototypes.json`.

## Hooks

Nothing to enable. The binary already classifies with the local embedder.

```bash
export DOWNSHIFT_MINILM=0
export DOWNSHIFT_MINILM_EMBED="python3 /path/to/harness-downshift/tools/minilm/embed_stdin.py"
```

The embed command must read stdin and print a JSON float array. A failing command falls back to hash.

## Beta exit

Tracked as **P4.9** in `docs/BETA-EXIT.md`.

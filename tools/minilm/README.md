# MiniLM semantic boost (optional)

Local **all-MiniLM-L6-v2** centroids trained from `benchmark/tasks.json`.
Used as a Jev/Laya-style decision layer: no LLM, only raises uncertain regex
classifications when embedding similarity agrees.

## Train / refresh prototypes

```bash
pip install sentence-transformers
python3 tools/minilm/train_prototypes.py
```

Commits target: `internal/semantic/data/prototypes.json`.

## Enable in hooks

```bash
export DOWNSHIFT_MINILM=1
export DOWNSHIFT_MINILM_EMBED="python3 /path/to/harness-downshift/tools/minilm/embed_stdin.py"
```

The embed command must read stdin and print a JSON float array.

## Beta exit

Tracked as **P4.9** in `docs/BETA-EXIT.md`.

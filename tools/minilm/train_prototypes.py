#!/usr/bin/env python3
"""Build MiniLM centroid prototypes from benchmark/tasks.json.

Requires: pip install sentence-transformers

Output: internal/semantic/data/prototypes.json (committed to the repo).
"""
from __future__ import annotations

import json
import sys
from collections import defaultdict
from pathlib import Path

MODEL = "sentence-transformers/all-MiniLM-L6-v2"


def main() -> int:
    root = Path(__file__).resolve().parents[2]
    tasks_path = root / "benchmark" / "tasks.json"
    out_path = root / "internal" / "semantic" / "data" / "prototypes.json"

    tasks = json.loads(tasks_path.read_text())
    try:
        from sentence_transformers import SentenceTransformer
    except ImportError:
        print("Install: pip install sentence-transformers", file=sys.stderr)
        return 1

    model = SentenceTransformer(MODEL)
    by_label: dict[str, list] = defaultdict(list)
    for row in tasks:
        label = row["label"].upper()
        by_label[label].append(row["prompt"])

    centroids = {}
    for label, prompts in by_label.items():
        vecs = model.encode(prompts, normalize_embeddings=True)
        dim = len(vecs[0])
        avg = [0.0] * dim
        for v in vecs:
            for i, x in enumerate(v):
                avg[i] += float(x)
        n = len(vecs)
        centroids[label] = [x / n for x in avg]

    payload = {"model": MODEL, "dim": len(next(iter(centroids.values()))), "centroids": centroids}
    out_path.parent.mkdir(parents=True, exist_ok=True)
    out_path.write_text(json.dumps(payload, indent=2) + "\n")
    print(f"wrote {out_path} ({payload['dim']}d, {len(centroids)} labels)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

#!/usr/bin/env python3
"""Train MiniLM centroids on benchmark/tasks.json and score benchmark/holdout.json.

Does not tune on the holdout. Prints tier accuracy and FRONTIER→MID for the
nearest-centroid classifier. The seed file is the only training source.
"""
from __future__ import annotations

import json
from collections import defaultdict
from pathlib import Path

import numpy as np
from sentence_transformers import SentenceTransformer

MODEL = "sentence-transformers/all-MiniLM-L6-v2"
ROOT = Path(__file__).resolve().parents[2]

TIER = {"TRIVIAL": "SMALL", "SIMPLE": "MID", "MEDIUM": "MID", "COMPLEX": "FRONTIER"}


def load(path: Path) -> list[dict]:
    return json.loads(path.read_text())


def centroids(model: SentenceTransformer, rows: list[dict]) -> dict[str, np.ndarray]:
    by: dict[str, list[str]] = defaultdict(list)
    for row in rows:
        by[row["label"].upper()].append(row["prompt"])
    out = {}
    for label, prompts in by.items():
        vecs = model.encode(prompts, normalize_embeddings=True)
        out[label] = np.mean(vecs, axis=0)
        out[label] /= np.linalg.norm(out[label])
    return out


def nearest(vec: np.ndarray, cents: dict[str, np.ndarray]) -> str:
    best, score = "", -1.0
    for label, c in cents.items():
        s = float(np.dot(vec, c))
        if s > score:
            best, score = label, s
    return best


def rates(rows: list[dict], pred: list[str]) -> dict:
    n = len(rows)
    tier_ok = 0
    frontier = mid = 0
    for row, p in zip(rows, pred):
        exp = row["label"].upper()
        if TIER[exp] == TIER[p]:
            tier_ok += 1
        if TIER[exp] == "FRONTIER":
            frontier += 1
            if TIER[p] == "MID":
                mid += 1
    return {
        "tasks": n,
        "tier_accuracy": tier_ok / n if n else 0,
        "frontier_to_mid": (mid / frontier) if frontier else 0,
        "frontier_total": frontier,
    }


def main() -> None:
    model = SentenceTransformer(MODEL)
    seed = load(ROOT / "benchmark" / "tasks.json")
    hold = load(ROOT / "benchmark" / "holdout.json")
    cents = centroids(model, seed)
    vecs = model.encode([r["prompt"] for r in hold], normalize_embeddings=True)
    pred = [nearest(v, cents) for v in vecs]
    report = rates(hold, pred)
    report["model"] = MODEL
    report["trained_on"] = "benchmark/tasks.json"
    report["evaluated_on"] = "benchmark/holdout.json"
    out = ROOT / "benchmark" / "minilm-holdout.json"
    out.write_text(json.dumps(report, indent=2) + "\n")
    print(json.dumps(report, indent=2))


if __name__ == "__main__":
    main()

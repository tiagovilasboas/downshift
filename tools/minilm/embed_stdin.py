#!/usr/bin/env python3
"""Read prompt from stdin; print JSON embedding array (normalized)."""
from __future__ import annotations

import json
import sys

MODEL = "sentence-transformers/all-MiniLM-L6-v2"


def main() -> int:
    text = sys.stdin.read()
    try:
        from sentence_transformers import SentenceTransformer
    except ImportError:
        print("[]", file=sys.stdout)
        return 1
    model = SentenceTransformer(MODEL)
    vec = model.encode([text], normalize_embeddings=True)[0]
    print(json.dumps([float(x) for x in vec]))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

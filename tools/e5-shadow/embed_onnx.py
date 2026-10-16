#!/usr/bin/env python3
# Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
# SPDX-License-Identifier: Apache-2.0
"""ONNX Runtime helper for intfloat/multilingual-e5-small (INT8 trial).

Reads one prompt on stdin, prints a JSON array of floats on stdout.
Model and tokenizer paths come from the environment (not argv) so the
Downshift hook can keep a fixed DOWNSHIFT_E5_SHADOW_CMD.

Environment:
  DOWNSHIFT_E5_ONNX_MODEL   — path to *.onnx (INT8 or FP32)
  DOWNSHIFT_E5_TOKENIZER    — HuggingFace tokenizer directory (optional if bundled)

Example:
  export DOWNSHIFT_E5_ONNX_MODEL=$HOME/.downshift/models/e5-small-int8.onnx
  export DOWNSHIFT_E5_TOKENIZER=$HOME/.downshift/models/e5-small-tokenizer
  export DOWNSHIFT_E5_SHADOW_CMD="python3 $(pwd)/tools/e5-shadow/embed_onnx.py"
"""

from __future__ import annotations

import json
import os
import sys


def main() -> int:
    model_path = os.environ.get("DOWNSHIFT_E5_ONNX_MODEL", "").strip()
    if not model_path:
        print("missing DOWNSHIFT_E5_ONNX_MODEL", file=sys.stderr)
        return 2
    try:
        import numpy as np
        import onnxruntime as ort
    except ImportError:
        print("install onnxruntime and numpy", file=sys.stderr)
        return 2

    prompt = sys.stdin.read()
    # E5 retrieval prefix for query encoding.
    text = f"query: {prompt}"

    tok_dir = os.environ.get("DOWNSHIFT_E5_TOKENIZER", "").strip()
    if tok_dir:
        try:
            from transformers import AutoTokenizer
        except ImportError:
            print("install transformers for tokenizer path", file=sys.stderr)
            return 2
        tokenizer = AutoTokenizer.from_pretrained(tok_dir)
        encoded = tokenizer(text, return_tensors="np", truncation=True, max_length=512)
        input_ids = encoded["input_ids"].astype(np.int64)
        attention_mask = encoded["attention_mask"].astype(np.int64)
        feeds = {"input_ids": input_ids, "attention_mask": attention_mask}
    else:
        print("DOWNSHIFT_E5_TOKENIZER required for this helper", file=sys.stderr)
        return 2

    sess = ort.InferenceSession(model_path, providers=["CPUExecutionProvider"])
    out = sess.run(None, feeds)
    # Mean pool last hidden state (common ST export); adjust if your graph differs.
    hidden = out[0]
    if hidden.ndim == 3:
        mask = attention_mask.astype(np.float32)[..., None]
        summed = (hidden * mask).sum(axis=1)
        denom = np.clip(mask.sum(axis=1), 1e-9, None)
        vec = summed / denom
    else:
        vec = hidden.reshape(1, -1)
    vec = vec[0]
    norm = np.linalg.norm(vec)
    if norm > 0:
        vec = vec / norm
    json.dump(vec.tolist(), sys.stdout)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

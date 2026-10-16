# E5 shadow embed helper

harness-downshift by Tiago de Carvalho Vilas Boas  
https://github.com/tiagovilasboas/downshift

Shadow-only path: set `DOWNSHIFT_E5_SHADOW=1` and point `DOWNSHIFT_E5_SHADOW_CMD` at a local embed helper. Production `ClassifyWithSemantic` / MiniLM is unchanged.

## Layout (user state dir)

| Path | Purpose |
|------|---------|
| `~/.downshift/models/e5-small-int8.onnx` | Quantized ONNX graph (not committed) |
| `~/.downshift/models/e5-small-tokenizer/` | HuggingFace tokenizer files |
| `~/.downshift/e5-prototypes.json` | Centroids (`PrototypeStore` JSON) built in labs |

## Hook env

```bash
export DOWNSHIFT_E5_SHADOW=1
export DOWNSHIFT_E5_ONNX_MODEL="$HOME/.downshift/models/e5-small-int8.onnx"
export DOWNSHIFT_E5_TOKENIZER="$HOME/.downshift/models/e5-small-tokenizer"
export DOWNSHIFT_E5_SHADOW_CMD="python3 /path/to/downshift/tools/e5-shadow/embed_onnx.py"
export DOWNSHIFT_E5_PROTOTYPES="$HOME/.downshift/e5-prototypes.json"
```

Observations land in hook telemetry under `e5_shadow` (no prompt text).

## INT8 validation (labs)

1. Export or download `intfloat/multilingual-e5-small` and quantize to INT8 with your chosen toolchain.
2. Compare cosine parity vs FP32 on a fixed sentence set before promoting centroids.
3. Measure cold vs warm latency under `hookctx` (1s embed budget); prefer a warm sidecar if p95 exceeds budget.

Quantization scripts stay in `downshift-labs`; do not commit large artifacts to this repo.

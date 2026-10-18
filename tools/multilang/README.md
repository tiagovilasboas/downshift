# Private multilingual evaluation runner

Downshift by Tiago Vilas Boas — https://github.com/tiagovilasboas/downshift

This offline development command measures a private, prelabelled fixture. It neither installs a model nor modifies runtime routing. Keep prompts, vectors, prototype training data, native-session metadata and detailed results in the private labs repository.

```sh
go run ./tools/multilang --input /private/m3-v1.json > /private/baseline.json
go run ./tools/multilang --input /private/m3-v1.json \
  --vectors /private/vectors.json --prototypes /private/prototypes.json > /private/candidate.json
go run ./tools/multilang --input /private/m3-v1.json --helper-probe /private/embed-helper
```

Input schema: an object with `items`, each with unique `id`, `language`, `prompt`, `expected_tier` (`small`, `mid`, `frontier`), and boolean `risk`. The vector JSON has `model` and `vectors` (item IDs to vectors); its model must exactly match the prototype store; the prototype JSON follows `semantic.PrototypeStore`. Centroids must come from a separate development corpus in the same embedding space. Fixtures and vectors are validated; reports contain IDs and metadata, never task text.

Baseline runs `core.ClassifyWithSemantic`. The candidate replaces its semantic neighbor lookup with precomputed multilingual vectors and development centroids, retaining the no-signal, confidence and monotonic-class restrictions. This measures classification, not a live model rewrite or executor success. Warm encoding latency must be added to the classifier's timing; precomputed vector timing alone is insufficient. `--helper-probe` invokes the existing bounded `semantic.CmdEmbedder` and reports failure rather than counting hash fallback as candidate success.

Guia inferencial / behaviour: fixed labels, independent development centroids and adoption rubric. Sensor computacional / behaviour: fixture hash, tier accuracy, coverage, unsafe recommendations and helper timeout. Sensor computacional / architecture fitness: dimensions and matching embedding-space requirement. Runtime adoption needs an independent routing safety and latency gate.

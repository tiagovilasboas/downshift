# Decision Intelligence

`harness-downshift` by Tiago de Carvalho Vilas Boas  
https://github.com/tiagovilasboas/downshift

This package adds an explainable, provider-agnostic advisory layer for routing.
It receives **structured** risk, sensitivity, budget, classifier-confidence and
reviewed-outcome signals. It does not receive or persist prompts, call an LLM,
perform network I/O, select a provider, or grant permissions.

`internal/decisionintelligence.Evaluate` is monotonic: it preserves or raises
the deterministic router's post-gate `BaseTier`; it never lowers it. Its output
has `Apply: false`, so deterministic policy remains the sole authority for
safety, permissions, budget enforcement and final model selection.

## Jev boundary

`JevAdapter` is a disabled-by-default extension point only. The repository has
no Jev SDK, account, API key, network request or runtime dependency. A future
adapter may provide advisory semantic signals in shadow mode, but must retain
the same structured contract and cannot override hard gates.

## Fowler contract

| Concern | Guia / sensor | Type | Eixo | Evidence |
| --- | --- | --- | --- | --- |
| No LLM or provider selection in routing | This contract and typed package | Guia | Architecture fitness | No dependency or I/O in package |
| Hard-gate preservation | Monotonic `Evaluate` and `Apply:false` | Guia | Behaviour | Unit tests prove no downgrade and advisory-only output |
| Sensitive/risky work | Review-required recommendation | Guia | Behaviour | Unit tests prove review signal and tier floor |
| Regression detection | `go test ./...`, `go vet ./...`, `git diff --check` | Sensor | Maintainability | CI/local command results |
| Runtime value | Prompt-free correlated telemetry plus reviewed feedback | Sensor | Behaviour | Existing JSONL and feedback loop; never treat rewrite emission as provider ACK |

The deterministic tests are **sensores computacionais**. A future security or
architecture review is a **sensor inferencial**, not an authorization bypass.

## Integration order

1. Run this layer in shadow mode beside `core.Route` and record only the
   recommendation metadata in the existing prompt-free event contract.
2. Compare its recommendations with explicit, reviewed outcomes; do not learn
   from unreviewed success/failure events.
3. Define a deterministic policy that consumes specific advisory fields, with
   an explicit budget gate and provider allowlist still evaluated afterwards.
4. Add a native harness acknowledgement before claiming the selected model was
   actually applied.

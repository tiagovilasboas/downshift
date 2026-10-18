# Operational State — Frente 1: DCI Telemetry & Stats Integration

- **Current Objective**: Oversee the epistemic contract and delivery of DCI telemetry and stats integration into `downshift stats` and `--export`.
- **Current Status**: Complete. 100% green tests in `cmd/downshift/...`, `internal/telemetry/...`, and `internal/codeintel/...`.
- **Active Task**: Delivery completed and verified.
- **Blockers**: None.
- **Epistemic Invariants**:
  1. Strictly OBSERVED telemetry: prompts evaluated, injections applied, abstentions by reason, context bytes emitted, observed latencies (p50, p95).
  2. Zero synthetic token counts or fictitious dollar savings.
  3. Clean JSON output under `dci` in `downshift stats --export`.
  4. Privacy & security: strict file permissions (0600) and zero persistence of prompts or source code.
- **Next Steps**:
  1. Report deliverable back to orchestrator / caller.

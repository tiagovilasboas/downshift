# Task List — Downshift Code Intelligence (DCI) MVP v0.1

> 📦 *Histórico compactado: 42 tarefas concluídas arquivadas em [completed-tasks-2026-10-18.md](archive/completed-tasks-2026-10-18.md)*

> 📦 *Histórico compactado: 64 tarefas concluídas arquivadas em [completed-tasks-2026-10-18.md](archive/completed-tasks-2026-10-18.md)*











  - [x] 12.2 Create `docs/design/code-intelligence.md` detailing architecture, epistemology, limits, and security.
  - [x] 12.3 Run full test suite (`go test ./internal/codeintel/...`, `go test ./cmd/downshift/...`, `gofmt`).
  - [x] 12.4 Commit, push, and summarize delivery.

## Frente 1: DCI Telemetry & Stats Integration
  - [x] F1.1 Oversee epistemic contract for DCI telemetry (prompts evaluated, injections applied, abstentions by reason, context bytes emitted, latencies p50/p95; zero synthetic tokens/dollars).
  - [x] F1.2 Coordinate CLI formatting in `cmd/downshift/cmd_stats.go` and `codeintel.PrintDCIStats` with `--dci`, `--detail`, and default views.
  - [x] F1.3 Ensure `--export` outputs clean JSON for DCI metrics under `dci` key in `telemetry.Summary`.
  - [x] F1.4 Verify all tests pass with `go test ./cmd/downshift/...`, `go test ./internal/telemetry/...`, and `go test ./internal/codeintel/...`.

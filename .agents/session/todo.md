# Task List — Downshift Code Intelligence (DCI) MVP v0.1

> 📦 *Histórico compactado: 42 tarefas concluídas arquivadas em [completed-tasks-2026-10-18.md](archive/completed-tasks-2026-10-18.md)*







  - [x] 8.2 Telemetry recording (observed durations, file counts, byte sizes; no prompts or code stored).
  - [x] 8.3 Unit and integration tests for service (`service_test.go`).
  - [x] 8.4 Commit and push Task 8.

- [ ] Task 9: CLI Commands & Configuration Management
  - [ ] 9.1 Implement `cmd/downshift/dci.go`: `status`, `enable`, `disable`, `doctor`, `inspect`, `benchmark`.
  - [ ] 9.2 Configuration storage in `.downshift/dci.json` (modes: off, observe, active).
  - [ ] 9.3 CLI tests (`cmd/downshift/dci_test.go`).
  - [ ] 9.4 Commit and push Task 9.

- [ ] Task 10: Claude Code UserPromptSubmit Hook
  - [ ] 10.1 Implement `cmd/downshift/dci_hook.go`: stdin JSON parsing (`UserPromptSubmit`), execution of DCI service within deadline.
  - [ ] 10.2 JSON output emission (`hookSpecificOutput.additionalContext`).
  - [ ] 10.3 Fail-open error handling (exit 0 on timeout, error, or abstention).
  - [ ] 10.4 Unit tests for hook (`dci_hook_test.go`).
  - [ ] 10.5 Commit and push Task 10.

- [ ] Task 11: Setup & Plugin Integration
  - [ ] 11.1 Update `cmd/downshift/setup_targets.go` to register `UserPromptSubmit` hook for claude-code (`downshift dci hook claude-code`).
  - [ ] 11.2 Update `hooks/hooks.json` to include `UserPromptSubmit`.
  - [ ] 11.3 Ensure idempotency and conflict prevention with existing hooks.
  - [ ] 11.4 Integration tests for setup merge (`setup_test.go`).
  - [ ] 11.5 Commit and push Task 11.

- [ ] Task 12: Benchmark, Security Review, Documentation & Validation
  - [ ] 12.1 Implement repeatable benchmark runner in `dci benchmark` with labeled fixtures.
  - [ ] 12.2 Create `docs/design/code-intelligence.md` detailing architecture, epistemology, limits, and security.
  - [ ] 12.3 Run full test suite (`go test ./internal/codeintel/...`, `go test ./cmd/downshift/...`, `gofmt`).
  - [ ] 12.4 Commit, push, and summarize delivery.

# Task List — Downshift Code Intelligence (DCI) v0.2

> 📦 *Histórico compactado: tarefas do MVP v0.1 e Frente 1 arquivadas em `archive/`*

## DCI v0.2: Multi-Language Symbols & Multi-Harness Hooks
- [x] **V02.1 (Fullstack - Multi-Lang Parser):** Implement `internal/codeintel/parse_lang.go` and wire in `parse.go`:
  - TypeScript/JavaScript (`.ts`, `.tsx`, `.js`, `.jsx`, `.mjs`, `.cjs`): functions, arrow functions, classes, class methods (`ClassName.method`), interfaces, types, enums with brace-depth `StartLine`..`EndLine` tracking.
  - Python (`.py`): module functions, classes, class methods (`ClassName.method`) with indentation-based `StartLine`..`EndLine` tracking.
  - C-style fallback (`.rs`, `.php`, `.java`, `.cs`) with brace-depth span estimation.
- [x] **V02.2 (AI / FinOps - Multi-Harness Hook & Budget):** Update `cmd/downshift/dci_hook.go` and `cmd/downshift/dci.go`:
  - Support `claude-code`, `cursor`, `opencode`, `codex`, and `generic` in `downshift dci hook <harness>`.
  - Support `--budget <bytes>` flag on `downshift dci enable [active|observe] [--budget=3500]` (clamped to safe bounds 512..16384 bytes).
  - Update `docs/design/code-intelligence.md`, `README.md`, and `docs/reference/capability-matrix.md`.
- [x] **V02.3 (AppSec - Parser & Hook Security):** Audit and add tests in `internal/codeintel/security_test.go`:
  - Test deeply nested braces, unclosed strings/braces, pathological lines (ReDoS resistance / linear-time scanning), and oversized/malformed hook payloads across all harnesses.
- [x] **V02.4 (QA - End-to-End Test Suite):** Add comprehensive tests in `internal/codeintel/parse_lang_test.go` and `cmd/downshift/dci_test.go`:
  - Verify multi-line `StartLine`..`EndLine` extraction for TS, JS, Python, Rust, PHP.
  - Verify `downshift dci hook` for `cursor`, `opencode`, `codex`, `claude-code`, and `--budget` flag.
  - Run full test suite, `go vet`, `gofmt`, push, open PR, and verify green CI.

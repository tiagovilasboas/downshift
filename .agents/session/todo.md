# Task List — Downshift Code Intelligence (DCI) v0.4

> 📦 *Histórico compactado: tarefas do MVP v0.1, Frente 1, v0.2 e v0.3 arquivadas em `archive/`*

## DCI v0.4: Reverse Call-Graph (Blast Radius) & Multi-Language Calls
- [ ] **V04.1 (Fullstack - Core Engine & CLI):**
  - Update `internal/codeintel/types.go`: add `Callers map[string][]string` to `InvertedIndex` and `ImpactRadius []string` to `Candidate`/`ContextPack`.
  - Update `internal/codeintel/parse_lang.go` and `internal/codeintel/parse.go`: extract lightweight calls and imports for TypeScript/JavaScript, Python, Rust, PHP, Java, and C# (reusing regex masks, capped at 16 calls per symbol and 32 imports per file).
  - Update `internal/codeintel/index.go`: in `RebuildPostings`, compute reverse call graph (`Callers` mapping each called symbol/function to list of caller files/symbols).
  - Update `internal/codeintel/rank.go` and `internal/codeintel/pack.go`: in `PackContext`, include a compact `Impact Radius` summary when top candidate symbols are called elsewhere in the codebase.
  - Update `cmd/downshift/dci.go`: add `downshift dci impact <symbol|file>` and show callers/impact in `dci inspect`.
- [x] **V04.2 (QA & AppSec - Tests, Hardening & Documentation):**
  - Add security and QA tests in `internal/codeintel/security_test.go`, `internal/codeintel/rank_test.go`, and `cmd/downshift/dci_test.go`:
    - Test multi-language call extraction (TS/Python/Rust).
    - Test reverse call-graph computation (`Callers`) and circular dependency resistance (A calls B, B calls A).
    - Test `downshift dci impact <target>` CLI command and JSON output.
    - Test context packing budget respect with impact radius annotations.
  - Update documentation in `docs/design/code-intelligence.md`, `ROADMAP.md`, and `README.md`.

# Operational State — DCI v0.4: Reverse Call-Graph (Blast Radius) & Multi-Language Calls

- **Current Objective**: Implement, test, document, and integrate DCI v0.4 (`feat/dci-v04-blast-radius-multilang-calls`):
  1. **Cross-Language Calls & Imports**: Extract lightweight outgoing function/method calls and imports in `internal/codeintel/parse_lang.go` and `internal/codeintel/parse.go` for TS/JS, Python, Rust, PHP, Java, and C#.
  2. **Reverse Call-Graph & Blast Radius Index**: Compute global reverse call mappings (`InvertedIndex.Callers: map[string][]string`) and `CalculateBlastRadius(symbol/file)` in `internal/codeintel/index.go` and `internal/codeintel/rank.go`.
  3. **Context Packaging & CLI**: Surface reverse call dependencies (Blast Radius / Impact) in `PackContext` (showing callers when modifying code) and `downshift dci inspect` / `downshift dci impact <target>`.
  4. **Security & QA Verification**: Verify nested call chains, circular dependencies, budget safety, and 100% test pass.
- **Current Status**: Task V04.2 completed (QA & AppSec test suites, hardening & documentation).
- **Branch**: `feat/dci-v04-blast-radius-multilang-calls`
- **Agents Assigned**: Senior Staff QA & AppSec finished; Senior Staff Fullstack integrating V04.1.
- **Blockers**: None.
- **QA & AppSec Verdict**: PASS (100% green under `go test -v -race ./internal/codeintel/... ./cmd/downshift/...`).

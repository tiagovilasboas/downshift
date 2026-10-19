# Task List — Downshift Code Intelligence (DCI) v0.3

> 📦 *Histórico compactado: tarefas do MVP v0.1, Frente 1 e v0.2 arquivadas em `archive/`*

## DCI v0.3: Go AST Call-Graph & Production-First Ranking
- [x] **V03.1 (Fullstack - Go AST Call-Graph & Ranking):** Edit `internal/codeintel/types.go`, `internal/codeintel/parse.go`, and `internal/codeintel/rank.go`:
  - Add `Calls []string` to `Symbol` and `Imports []string` to `Document` in `types.go`.
  - In `parseGoAST` (`parse.go`), extract unique outgoing function/method calls (`*ast.CallExpr`) per `*ast.FuncDecl` (skipping Go built-ins, capped at 16 calls per symbol) and package imports (`node.Imports`, capped at 32).
  - In `RankCandidates` (`rank.go`), apply Production-vs-Test file weighting (`0.65x` on `_test.go` when query has no test/bench terms; `+2.0` boost when query asks for test/bench) and Call-Graph Co-Reference Boost (`+2.0` when a candidate defines a symbol called by top-1 candidate or calls a matched symbol of top-1 candidate).
- [x] **V03.2 (AI / FinOps - `dci inspect --json` & Docs):** Edit `cmd/downshift/dci.go`, `docs/design/code-intelligence.md`, and `README.md`:
  - Support `downshift dci inspect [--json] <prompt>` emitting structured JSON for automation/inspection and showing symbol `Calls` in text output.
  - Document Go AST Call-Graph Co-Reference Boosting, Production-First Ranking, and `dci inspect --json`.
- [x] **V03.3 (AppSec - Call-Graph & AST Security):** Edit `internal/codeintel/security_test.go`:
  - Verify deeply nested `*ast.CallExpr` chains (e.g. 500 chained calls `a().b().c()...`) and large import blocks do not cause stack overflow or blow up index cache size (verify `Calls` cap <= 16 and `Imports` cap <= 32).
- [x] **V03.4 (QA - Call-Graph & Ranking Test Suite):** Edit `internal/codeintel/rank_test.go` and `cmd/downshift/dci_test.go`:
  - Test Go AST `Symbol.Calls` and `Document.Imports` extraction.
  - Test Production-First Ranking (verify `service.go` ranks above `service_test.go` for non-test queries, and `service_test.go` ranks higher when query includes `"test"`).
  - Test Call-Graph Co-Reference Boost between caller and callee files.
  - Test `downshift dci inspect --json <prompt>`.

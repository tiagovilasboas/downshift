# Operational State — DCI v0.3: Go AST Call-Graph & Production-First Ranking

- **Current Objective**: Implement, test, secure, and document DCI v0.3 (`feat/dci-v03-go-callgraph-ranking`):
  1. Go AST Call-Graph (`Symbol.Calls`) and Package Import (`Document.Imports`) extraction in `internal/codeintel/types.go` & `parse.go`.
  2. Call-Graph Co-Reference Boost (`+2.0`) and Production-vs-Test Ranking (`0.65x` on test files for non-test queries, `+2.0` boost on test queries) in `internal/codeintel/rank.go`.
  3. `downshift dci inspect --json` and call-graph visualization in `cmd/downshift/dci.go` + documentation updates.
  4. Security & QA verification.
- **Current Status**: Complete. 100% green tests in `internal/codeintel/...` and `cmd/downshift/...`.
- **Branch**: `feat/dci-v03-go-callgraph-ranking`
- **Blockers**: None.
- **QA Verdict**: PASS.


# Evidence Report — Downshift Code Intelligence (DCI) v0.4

**Date:** 2026-10-19  
**Author:** Tiago Vilas Boas  
**Repository:** https://github.com/tiagovilasboas/downshift  
**Commits:** [`d3807545`](https://github.com/tiagovilasboas/downshift/commit/d3807545), [`0d141f76`](https://github.com/tiagovilasboas/downshift/commit/0d141f76) (PR [#179](https://github.com/tiagovilasboas/downshift/pull/179))  
**Harness / Hook Events:** `claude-code`, `cursor`, `opencode`, `codex`, `generic` (`UserPromptSubmit`)  
**Status:** Implemented, verified with green CI, 100% `-race` test pass, and reproducible benchmarks  

---

## 1. What was delivered in DCI v0.4

DCI v0.4 introduces **Reverse Call-Graph Indexing (*Blast Radius* / Impact Analysis)** and **Multi-Language Outgoing Calls & Imports Extraction** directly in pure Go standard library (zero CGo, zero Tree-sitter, zero external dependencies):

1. **Multi-Language Outgoing Calls & Imports (`internal/codeintel/parse_lang.go`, `parse.go`)**:
   - Pure Go regex/lexical AST parsers extracting outgoing function and method calls (`Symbol.Calls`, capped at 16 calls per symbol, excluding built-ins and keywords) and package imports (`Document.Imports`, capped at 32 imports per file) across **TypeScript/JavaScript, Python, Rust, PHP, Java, and C#**.
   - Proactive loop-level cap defense preventing memory exhaustion or buffer bloat on pathological files.

2. **Inverted Call Index & Reverse Call-Graph (`internal/codeintel/index.go`, `types.go`)**:
   - Computes `InvertedIndex.Callers` (`map[string][]string`) during `RebuildPostings`, inverting all outgoing calls into reverse caller mappings (`docPath:symbolName`).
   - Callers per symbol are strictly capped at 32 entries and sorted deterministically.
   - Helper method `(idx *InvertedIndex) GetCallers(symbolName string) []string` supports exact symbol matching and method suffix fallback.

3. **Downstream Blast Radius in Context Packaging (`internal/codeintel/pack.go`, `service.go`)**:
   - Added `ImpactRadius` tracking to `Candidate` and `ContextPack`.
   - Structural sketch includes compact blast radius annotations (e.g. `(Impact Radius: called by pkg/auth.go, pkg/router.go)`, capped at 4 callers), alerting coding agents to downstream callers before modifying signatures.
   - Strictly respects configurable byte budget (`--budget=N`, default 3,500 bytes UTF-8; clamped between 512 and 16,384).

4. **CLI Impact Command & Inspection (`cmd/downshift/dci.go`)**:
   - Subcommand `downshift dci impact [--json] <target>` queries reverse callers and dependent files for any symbol or file.
   - Subcommand `downshift dci inspect [--json]` includes `impact_radius` in structured JSON and human-readable text.

---

## 2. Empirical Benchmarks & Performance Verification

Ran on Downshift repository (Apple Silicon / macOS) on the `main` branch (`c08320db`):

```text
$ ./downshift dci doctor
Checking Downshift Code Intelligence (DCI) Health:
  [PASS] Repository Root: /Users/tiago.boas/Github/downshift
  [PASS] Config Mode: off (Context Budget: 3500 bytes)
  [PASS] Discovery: 881 files discovered in 95.42ms
  [PASS] Cache File Permissions: 0600
  [PASS] Cold Index Latency: 320.27ms (budget <= 2500ms)

DCI Doctor: All critical checks passed.

$ ./downshift dci benchmark
=== Running Downshift Code Intelligence (DCI) Benchmarks ===
Cold Index Latency:    295.56ms (Budget <= 2500ms) -> [PASS]
Warm Sync Latency:     18.12µs  (Budget <= 300ms)  -> [PASS]

Test Queries:
  Greeting Hi           : latency=1.00ms applied=false bytes=0    -> [PASS]
  Greeting Bom Dia      : latency=204µs  applied=false bytes=0    -> [PASS]
  Chatter Thanks        : latency=518µs  applied=false bytes=0    -> [PASS]
  Code Query Auth       : latency=5.64ms applied=true  bytes=568  -> [PASS]
  Code Query Service    : latency=4.93ms applied=true  bytes=2062 -> [PASS]

Warm Path p95 Latency: 5.64ms (Budget <= 300ms) -> [PASS]
Benchmark Result: ALL TESTS PASSED within epistemic budgets.
```

### Real Blast Radius CLI Verification
```text
$ ./downshift dci impact Tokenize
=== DCI Impact Blast Radius ===
Target:        Tokenize
Target Type:   Symbol
Callers/Deps:  7

  - cmd/downshift/dci.go:runDCIInspect
  - internal/codeintel/parse.go:ParseFile
  - internal/codeintel/rank.go:computeWeightedTF
  - internal/codeintel/rank.go:findDocMatchedSymbols
  - internal/codeintel/rank_test.go:TestRankCandidates_CallGraphCoReferenceBoost
  - internal/codeintel/rank_test.go:TestRankCandidates_ProductionFirstVsTestFiles
  - internal/codeintel/service.go:Service.ProcessPrompt
```

---

## 3. Security, Hardening & Limits Verification (AppSec Sign-Off)

Reviewed by Senior Staff AppSec & QA in `internal/codeintel/security_test.go`:
- **Circular Call Graphs & Infinite Recursion:** `TestSecurity_CircularCallGraphsAndPathologicalMultiLangLimits` constructs a 1,000-document cyclic chain (`CycleFunc_0000 -> CycleFunc_0001 -> ... -> CycleFunc_0999 -> CycleFunc_0000`). `RebuildPostings` finishes in <250ms and cyclic graph traversal does not deadlock, blow the stack, or recurse infinitely.
- **Hotspot Caller Capping:** Injects 50 distinct callers to a single target function (`TargetHotspot`). Validates that `idx.GetCallers("TargetHotspot")` enforces a strict hard cap of **<= 32 callers** and is sorted deterministically in alphabetical order.
- **Pathological Multi-Language Stress:** Files containing 100 imports and 200 calls across TypeScript, Python, and Rust strictly enforce `Document.Imports <= 32` and `Symbol.Calls <= 16`.
- **Zero Prompt / Code Persistence:** No user prompts, raw file snippets, or conversational text are saved to disk. Cache file permissions remain strictly `0600` on `.downshift/cache/codeintel.json`.

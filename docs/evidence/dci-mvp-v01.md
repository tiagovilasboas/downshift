# Evidence Report — Downshift Code Intelligence (DCI) MVP v0.1

**Date:** 2026-10-18  
**Author:** Tiago Vilas Boas  
**Repository:** https://github.com/tiagovilasboas/downshift  
**Commit:** [`6fb96043`](https://github.com/tiagovilasboas/downshift/commit/6fb96043b1b9e1ddb51f7df9ccee70e5ba07c06d) (PR [#169](https://github.com/tiagovilasboas/downshift/pull/169))  
**Harness / Hook Event:** Claude Code `UserPromptSubmit`  
**Status:** Implemented, verified with green CI and repeatable benchmarks  

---

## 1. What was built

Downshift Code Intelligence (DCI) MVP v0.1 adds an independent code intelligence and structural context optimization capability to Downshift. It automatically analyzes user prompts, extracts relevant symbols and source file references via Go AST and BM25, and injects a compact structural sketch into the harness before the agent begins working.

### Key Architectural Invariants
1. **Zero External Runtimes & Pure Go:** Zero Python, Rust, Tree-sitter, Ctags, or LSP daemons. 100% standard library (`go/parser`, `go/ast`, `go/token`, `crypto/sha256`, `os/exec`).
2. **Zero Runtime LLM Calls:** No neural embeddings, no vector databases, no recursive meta-prompting loops. Everything is deterministic.
3. **Strict Domain Isolation:** `internal/codeintel/` does not import `internal/core/` or `internal/classify/`, and vice versa. Model routing decisions are completely unaffected.
4. **Fail-Open Semantics:** Any error, timeout, or missing relevance immediately exits 0 without output, leaving normal agent execution unaffected.
5. **Zero-Persistence Privacy:** Source code snippets, user prompts, and conversation content are **never saved to disk or logs**. Only file metadata (relative paths, hashes, term counts, line numbers) is stored in the cache.

---

## 2. Benchmark & Latency Measurements

Measured on macOS (Apple Silicon) and validated on Linux x86_64 runners in GitHub Actions CI:

```text
=== Running Downshift Code Intelligence (DCI) Benchmarks ===
Cold Index Latency:    239.74ms (Budget <= 2500ms) -> [PASS]
Warm Sync Latency:     209ns    (Budget <= 300ms)  -> [PASS]

Test Queries:
  Greeting Hi           : latency=1.12ms applied=false bytes=0    -> [PASS]
  Greeting Bom Dia      : latency=156µs  applied=false bytes=0    -> [PASS]
  Chatter Thanks        : latency=456µs  applied=false bytes=0    -> [PASS]
  Code Query Auth       : latency=5.37ms applied=true  bytes=568  -> [PASS]
  Code Query Service    : latency=4.68ms applied=true  bytes=2801 -> [PASS]

Warm Path p95 Latency: 5.37ms   (Budget <= 300ms)  -> [PASS]
Benchmark Result: ALL TESTS PASSED within epistemic budgets.
```

### Retrieval Speedup
Microbenchmarks in `internal/codeintel/benchmark_test.go`:
- Unindexed brute-force lexical scan: **509 µs/op**
- Indexed BM25 retrieval: **104 µs/op** (~5x speedup)
- Context Packager assembly: **1.0 µs/op**

---

## 3. Epistemic Stance & Truthful Metrics

In accordance with Downshift's epistemic guidelines:
- **Observed Metrics:** Actual file counts (873 files indexed), output bytes (568 to 2801 bytes), and measured wall-clock latencies (<6 ms warm path).
- **No Fictitious Token Claims:** We do not claim arbitrary percentage reductions in model token invoices without empirical billing records. Context packs enforce a hard budget of **3,500 bytes UTF-8**.
- **Abstention as a Feature:** Conversational chatter, short greetings, and out-of-domain queries abstain immediately without injecting context, preserving token bandwidth.

---

## 4. Security & Privacy Audit (AppSec Sign-Off)

Reviewed by Senior Staff AppSec:
- **Path Traversal:** `ValidatePath` enforces absolute containment within repository root using `filepath.Clean` and `filepath.Rel`. Rejects null bytes and traversal escapes (`../`).
- **Symlink Protection:** Symlinks are checked with `os.Lstat` and skipped in file discovery, preventing arbitrary file reads outside the workspace.
- **Secret Scanning:** Automatic exclusion of `.env`, `.pem`, `.key`, `id_rsa`, and pattern scanning for AWS, GitHub, Stripe, and Slack tokens.
- **Cache Permissions:** Cache directory strictly created with `0700` (`rwx------`) and JSON metadata files written with `0600` (`rw-------`).
- **Prompt Injection Defense:** Injected blocks are marked with unambiguous delimiters (`=== DCI UNTRUSTED REPOSITORY CONTEXT ===`) and explicit security notices commanding models to treat content strictly as passive data.

---

## 5. Verification Commands

Reproduce all tests and benchmarks locally:
```bash
# Run unit and integration tests
go test -v -count=1 ./internal/codeintel/...
go test -v -count=1 ./cmd/downshift -run DCI

# Run health diagnostics
./downshift dci doctor

# Run benchmark suite
./downshift dci benchmark

# Test prompt inspection (dry-run)
./downshift dci inspect "process payment recurrence"
```

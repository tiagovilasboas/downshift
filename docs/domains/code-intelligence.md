# Domain: Code Intelligence (DCI)

Low-latency, privacy-first deterministic structural code intelligence and prompt context injection. Off by default (`downshift dci enable` turns it on); the measured evidence is latency and context bytes, not answer quality.

Downshift Code Intelligence (DCI) is a high-speed, zero-runtime subsystem that parses repository structure, builds incremental in-memory AST and symbol indexes, computes downstream blast radiuses, and injects compact, relevant code sketches directly into user prompts across coding harnesses (`claude-code`, `cursor`, `opencode`, `codex`, and `generic`).

---

## 1. Architectural Principles

1. **Zero External Runtime**: Written entirely in pure Go. Requires zero Python, Node, Rust, tree-sitter C-bindings, external daemons, or remote LLM calls.
2. **Strict Domain Decoupling**: DCI (`internal/codeintel`) operates completely independently from routing, classifier rules, and adapters (`internal/core`, `internal/classify`). Neither imports the other.
3. **Sub-Millisecond Warm Path**: Cold indexing finishes in under 2,500ms; warm path incremental retrieval executes in under 10ms (p95 < 10ms).
4. **Epistemic Honesty**: Telemetry records empirical observations only (indexed files, candidate counts, durations in milliseconds, context byte sizes). DCI never invents synthetic token counts or hypothetical dollar savings.
5. **Fail-Open by Design**: Any timeout, syntax issue, oversized payload (1 MiB stdin cap), or unexpected error exits 0 with zero context injected, guaranteeing zero disruption to agent execution.
6. **Privacy & Security First**: User prompts and raw file contents are never written to disk or logs. Files matching secret patterns (`.env*`, `*.key`, `*.pem`, `id_rsa`, AWS, Stripe, GitHub tokens) are automatically skipped during discovery.

---

## 2. Structural Pipeline

```
[User Prompt ("prompt" | "task" | "user_message")]
       │
       ▼ (via `downshift dci hook <harness>`)
[1. Security & Discovery Gate]
       ├── Scans via `git ls-files -z` (with WalkDir fallback)
       ├── Excludes binaries, vendor directories, and secrets
       └── Enforces safety caps: max 2,000 files, 512 KiB/file, 16 MiB total run
       │
       ▼
[2. AST Symbol Extraction & Incremental Index]
       ├── Go AST (`go/parser`): extracts function call-graph (Symbol.Calls) & package imports
       ├── Multi-Language Regex/AST: TypeScript/JavaScript, Python, Rust, PHP, Java, C#
       ├── Incremental change detection via SHA-256 hashes
       └── In-memory inverted index + disk cache at `.downshift/dci_index.json` (0600 permissions)
       │
       ▼
[3. BM25 Ranking & Call-Graph Boost]
       ├── BM25 scoring (k1=1.2, b=0.75; weights: symbol=5, filename=3, content=1)
       ├── Production-First Ranking: 0.65x penalty on `_test.go` for code prompts; +2.0 boost for test prompts
       └── Call-Graph Co-Reference Boost: +2.0 boost for caller/callee file pairs
       │
       ▼
[4. Reverse Call-Graph & Blast Radius]
       ├── Inverts call graph into `InvertedIndex.Callers`
       └── Traces downstream callers (capped at 32 per symbol, recursion-safe)
       │
       ▼
[5. Relevance Filter & Abstention Engine]
       ├── Conversational Abstention: Greetings ("hi", "bom dia"), gratitude ("thanks"), chatter → Abstain
       └── Requires explicit symbol/path match OR >=2 relevant query terms
       │
       ├── (Abstain) ──► Exit 0 (zero context injected, saves tokens)
       └── (Pass)    ──► Context Packager
                               │
                               ▼
[6. Packaging & Boundary Defense]
       ├── Clamped budget: default 3,500 bytes (configurable 512 to 16,384 bytes)
       ├── Wraps context in `=== DCI UNTRUSTED REPOSITORY CONTEXT ===` markers
       └── Emits hook output (`hookSpecificOutput` and `additionalContext`)
```

---

## 3. Production-First Ranking & Blast Radius

- **Production-First Weighting**: Development tasks often touch implementation files while referencing test names. Non-test queries receive a `0.65x` demotion on test files, preventing test fixtures from crowding out production code. When a prompt explicitly queries tests ("test", "spec", "coverage"), test files receive a `+2.0` score boost.
- **Call-Graph Co-Reference**: Files that define functions called by the primary candidate receive an automatic `+2.0` score boost, bringing upstream interfaces and downstream implementations into the same context sketch.
- **Downstream Blast Radius**: Refactoring a function signature requires knowing who calls it. DCI traces callers across the codebase, packaging a concise "Impact Radius" note that warns the agent of affected call sites before code is rewritten.

---

## 4. Multi-Harness Hook Integration

DCI integrates with major coding harnesses via transparent hooks:

| Harness | Hook Command | Integration Point | Output Format |
|---|---|---|---|
| **Claude Code** | `downshift dci hook claude-code` | `UserPromptSubmit` | `hookSpecificOutput.user_message` |
| **Cursor** | `downshift dci hook cursor` | `beforePromptSubmit` | `additionalContext` + `text` |
| **OpenCode** | `downshift dci hook opencode` | `user_prompt_submit` | `additionalContext` |
| **Codex** | `downshift dci hook codex` | `PromptSubmit` | `additionalContext` |
| **Generic** | `downshift dci hook generic` | JSON stdin / stdout | Generic JSON envelope |

---

## 5. DCI Management Commands

- `downshift dci status`: View active mode (`off`, `observe`, `active`), indexed file counts, symbol totals, and cache freshness.
- `downshift dci enable active [--budget=3500]`: Enable live context injection (or `observe` to evaluate latencies and hit rates without modifying prompts).
- `downshift dci disable`: Deactivate DCI context injection.
- `downshift dci inspect [--json] "<prompt>"`: Dry-run the complete pipeline for a prompt, displaying candidate scores, blast radius notes, and the formatted context pack.
- `downshift dci impact [--json] <target>`: Query downstream caller dependencies and blast radius for a given symbol or file.
- `downshift dci doctor`: Validate git discovery, directory permissions, cache validity, and end-to-end latency.
- `downshift dci benchmark`: Run local indexing, AST parsing, and BM25 retrieval latency benchmarks.
- `downshift stats --dci`: Display empirical telemetry including prompt evaluation count, injection rate, abstention breakdown, and p50/p95 latency metrics.

---

## 6. Deep References

- [DCI Design & Specification](../design/code-intelligence.md): Comprehensive architectural specification, security boundaries, and telemetry schemas.
- [DCI v0.1 MVP Evidence](../evidence/dci-mvp-v01.md): Empirical benchmark runs, indexing times, and retrieval latencies across test repositories.

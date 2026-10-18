# Architectural Decisions — Downshift Code Intelligence (DCI) MVP v0.1

## ADR-001: Strict Domain Isolation
- `internal/codeintel` is completely decoupled from `internal/core`, `internal/classify`, and adapters.
- Code Intelligence (context optimization) and Model Routing are orthogonal concerns.
- DCI does NOT alter routing decisions, does not run in `PreToolUse`, and does not alter subagent models.
- Neither package imports the other.

## ADR-002: Zero External Runtimes & Determinism
- Pure Go standard library: `go/parser`, `go/ast`, `go/token`, `crypto/sha256`, `os/exec`.
- No embeddings, vector databases, tree-sitter, LSP daemons, or LLM calls.
- Deterministic tokenization (camelCase, PascalCase, snake_case, acronyms, path separators).
- Deterministic BM25 ranking (k1=1.2, b=0.75, tie-breaking by Score DESC, Path ASC, StartLine ASC).

## ADR-003: Privacy, Security & Fail-Open
- Read-only execution against validated repository root.
- No network, no shell interpolation, no symlink traversal.
- Zero persistence of user prompts, source code snippets, or generated context.
- Index cache contains only relative paths, SHA-256 hashes, term frequencies, and symbol positions (mode 0600/0700).
- Any error, timeout, or lack of relevance immediately fails open (exit 0, empty injection).
- Injected context is clearly delimited as untrusted repository data with prompt injection disclaimers.

## ADR-004: Claude Code Auto-Injection Protocol
- Hook: `UserPromptSubmit` via `downshift dci hook claude-code`.
- Schema: JSON on stdin with `hook_event_name`, `cwd`, `session_id`, `prompt`.
- Response: `{"hookSpecificOutput":{"hookEventName":"UserPromptSubmit","additionalContext":"..."}}`.
- Modes: `off` (default), `observe` (metrics only), `active` (injects when eligible).

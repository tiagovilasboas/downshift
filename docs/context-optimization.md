# Native Context Compressor & Context Sensor

Downshift includes an in-process, zero-dependency **Native Context Compressor** (`internal/compressor`) and event-driven **Context Sensor** (`internal/sensor`) to govern token consumption without altering critical diagnostics or introducing external runtimes.

---

## 1. Architectural Principles

1. **Enabled by Default & Zero External Runtime**: Written purely in Go. Runs in-process by default with zero Python, Rust, Node, network or LLM dependencies.
2. **Deterministic & Fail-Open**: Squelches repetitive success output only. **Never alters errors, stack traces, non-zero exit codes, or failing test suites.**
3. **Observability vs. Transformability**: Distinguishes observational telemetry (e.g. `PostToolUse` telemetry) from transformational filters (command pipes or tool proxies).
4. **Epistemic Precision**: Metrics clearly distinguish **Observed** counts from **Estimated** heuristics and **Unavailable** harness channels.
5. **Privacy First**: Sensor stores only session hashes, format classifications, and byte aggregates. **Prompts, private code, and sensitive outputs are never persisted.**

---

## 2. Supported Formats & Compression Rules

| Format | Detection Heuristic | Safe Action | Fail-Open Guarantee |
|---|---|---|---|
| `go test` | `=== RUN`, `--- PASS:`, `ok` | Aggregates 100% passing suites | Any `FAIL`, trace or build error is preserved verbatim |
| `git status` | `On branch`, `nothing to commit` | Compacts clean status or repetitive clean trees | Preserves branch, conflicts and uncommitted changes |
| `git log` | `commit [0-9a-f]{40}` | Preserves recent commits; drops repetitive author metadata | Full commit hashes preserved |
| `search` (grep/rg) | `path:line: content` | Groups results by file; caps excessive lines with summary | Preserves matching line numbers and snippets |
| `file_listing` | `find`, `ls -R`, tree structures | Summarizes large tree listings | Preserves root and directory hierarchy |
| `logs` | Repeating log line prefixes | Collapses consecutive identical lines (`[Repeated N times]`) | Preserves first and last timestamp + all error logs |

---

## 3. Modes of Operation

- **`off`**: Pass-through mode; returns output completely untouched.
- **`observe`** (*Default for runtime hooks*): Computes byte savings and format classification without modifying output.
- **`safe`**: Applies deterministic compression only when format is verified and no error signals are detected.

---

## 4. CLI Reference

```bash
# Check status of context optimization (enabled by default)
downshift context status

# List providers and harness capabilities
downshift context providers

# Run diagnostics
downshift context doctor

# Re-enable native compressor if previously disabled
downshift context enable

# Pipe tool output through native compressor
cat verbose_test_output.txt | downshift context compress safe

# Inspect token savings and sensor observations
downshift context metrics

# Compare empirical benchmark scenarios
downshift context benchmark

# Disable and revert settings
downshift context disable
```

---

## 5. Harness Capability Matrix

| Harness | Telemetry Channel | Rewrite / Transform Channel | Mechanism |
|---|---|---|---|
| **Claude Code** | `PostToolUse` (Observes duration, tool name & bytes) | Explicit pipe or agent instructions | In-process sensor records volume; hook cannot rewrite output |
| **Cursor** | Telemetry logs | Agent instructions | Instruction-guided deterministic compaction |
| **Codex** | Hook payloads | Direct tool execution | In-process Go filter |
| **Antigravity** | Session lifecycle | Direct filter | Zero-dependency native engine |

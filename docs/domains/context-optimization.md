# Domain: Context Optimization & Compression

In-process context sensing, native output compaction, session policy governance, and companion integration.

Context bloat degrades agent reasoning quality and accelerates token exhaustion. Downshift approaches context optimization through multiple layers: an event-driven **Context Sensor**, an in-process **Native Context Compressor**, session-level execution policies (`opusplan`), and clean companion integrations with external proxies (e.g., Headroom).

---

## 1. Architectural Principles

1. **Zero External Runtime**: The native compressor (`internal/compressor`) and sensor (`internal/sensor`) are written in pure Go, requiring no Python, Rust, Node, external daemons, or remote LLM calls.
2. **Fail-Open & Diagnostic Preservation**: Never modifies or squelches error outputs, stack traces, compiler failures, non-zero exit codes, git commit logs, grep search matches, or directory trees.
3. **Observability vs. Transformation**: In harnesses like Claude Code, hooks cannot rewrite tool output in flight. Downshift honestly operates in **observe mode**, recording real tool exit codes and byte volumes without fabricating token reductions. The one transforming path is opt-in and off by default: `downshift context enable native --exec` lets the PreToolUse hook rewrite an allowlisted shell command (`go test`, `git status`, no shell metacharacters) to `downshift exec -- <command>`, which runs it, keeps the exit code and compacts only clean output (exit 0, empty stderr, the command's own format). Policy: `internal/execwrap`; per-harness status: [harness matrix](../reference/harness-matrix.md#shell-command-rewrite-downshift-exec-opt-in-2026-10-19). Its metrics are measured bytes; the token figure is labelled estimated (bytes / 4).
4. **Epistemic Honesty**: Downshift measures context reduction in **bytes**. It does not claim synthetic token counts or hypothetical dollar savings for context compaction unless backed by provider-level token accounting.
5. **Prompt Privacy**: Sensor records store format classifications, byte counters, and session hashes only. Prompts, source code, and tool stdout are never persisted.

---

## 2. Supported Formats & Compaction Heuristics

The native compressor evaluates tool stdout line-by-line and applies safe squelching only when the entire output represents an unambiguous clean run:

| Format | Detection Heuristic | Safe Action | Fail-Open Guarantee |
|---|---|---|---|
| `go test` | Non-empty lines match `ok <pkg>` | Collapses verbose passing packages into a concise summary | Any `FAIL`, compilation error, panic, or non-zero exit preserves the entire output verbatim |
| `git status` | `On branch`, `nothing to commit` | Compacts clean branch status | Preserves untracked files, modified paths, merge conflicts, and uncommitted diffs |
| `git log` | `commit [0-9a-f]{40}` | Preserved verbatim | Never summarized; commit messages and SHAs are preserved intact |
| `search` (grep/rg) | `path:line: content` | Preserved verbatim | Never truncated; search results remain fully intact |
| `file_listing` | `find`, `ls -R`, tree output | Preserved verbatim | Directory and file paths remain fully intact |
| `logs` | Repeating log line prefixes | Collapses consecutive identical lines (`[Repeated N times]`) | Preserves timestamps, error levels, and unique warning lines |

---

## 3. Operating Modes

- **`off`**: Context optimization disabled; outputs pass through completely untouched.
- **`observe`** (Default runtime): Evaluates formats and calculates byte reductions in-memory, updating sensor telemetry without mutating stdout.
- **`safe`**: Activates stream compaction on the CLI only when `--exit 0` is explicitly provided and the format passes all safety checks. If exit is non-zero, raw input is preserved verbatim.

---

## 4. Session Policy Governance (Claude Code)

During `downshift setup`, Downshift inspects `~/.claude/settings.json` and configures optimal token-saving policies for unset keys (can be skipped with `--no-session-policy`):

- **`"model": "opusplan"`**: Directs Opus to create high-level plans while delegating execution steps to Sonnet, significantly lowering base session token burn.
- **Plan Mode**: Enforces explicit planning milestones before broad file modifications.
- **You Should Know Mod** (opt-in, `downshift setup --you-should-know`): Enables Claude Code's built-in `cc-plugin-you-should-know@builtin` advisory mod. Each check is a billed background request, so setup only suggests it.
- **Cache-Tax Mod**: Bundled inside the Downshift Claude Code plugin (`hooks/cache-tax/`). Alerts when cold prompt cache read costs approach threshold limits and offers `/keepwarm` to maintain warm caches across idle intervals.

---

## 5. Companion Architecture

Downshift respects domain boundaries by integrating with companions rather than wrapping or re-implementing them:

- **Headroom (Default Companion)**: A local compression proxy for terminal sessions (`headroom wrap claude`). When present on `PATH`, `downshift context enable` detects it, and `downshift doctor` registers the companion so savings are not attributed to routing alone. Downshift never installs or executes Headroom automatically.
- **Recommended Mods**:
  - **Token Weather**: Displays context window utilization above the prompt.
  - **Replay Theater**: Step through the previous turn's file modifications diff-by-diff without model calls.
  - **reflect-mod**: Transforms operator corrections into persistent `CLAUDE.md` instructions using a lightweight Haiku call.

---

## 6. Context CLI Reference

- `downshift context status`: Show context optimization mode, active provider, and observed metrics.
- `downshift context enable [native|headroom]`: Enable the native compressor or record a companion provider.
- `downshift context disable`: Revert context optimization to raw pass-through.
- `downshift context doctor`: Verify local compression engine, provider availability, and harness configuration.
- `downshift context metrics`: Inspect observed command counts, format classifications, and byte aggregates.
- `downshift context compress`: Stream compaction utility for CLI pipelines (requires `--exit 0` in safe mode).
- `downshift context benchmark`: Benchmark compaction efficiency on versioned test fixtures (measures original bytes, reduced bytes, and byte savings).

---

## 7. Deep References

- [Context Optimization Design](../design/context-optimization.md): Architectural specification, sensor event formats, and safety policies.
- [Companions Guide](../guides/companions.md): Complete guide to Headroom, recommended Claude Code mods, and installation instructions.
- [Claude Code Integration Guide](../guides/claude-code.md): Deep-dive into session policies, opusplan mechanics, and baseline inference.

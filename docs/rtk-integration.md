# Optional RTK integration

This guide applies to harness-downshift by Tiago de Carvalho Vilas Boas:
https://github.com/tiagovilasboas/downshift

RTK is an optional companion for compacting supported shell-command output.
Downshift continues to route subagent model and reasoning effort; it does not
invoke RTK or depend on it at runtime. Keep these concerns in their respective
harness hooks.

## Enable for this project

Install RTK using its official instructions, then run from this repository:

```bash
rtk init --codex
```

This project-scoped setup adds RTK guidance to `AGENTS.md` through `RTK.md`.
On RTK 0.44.2, the Codex integration is instruction-based: it does not install
a Codex command hook. The agent uses supported commands such as `rtk git`,
`rtk rg`, and `rtk go test` explicitly. It does not modify Downshift's
`spawn_agent` hook or the separate shell safety hook.

Remove only this project's RTK setup with:

```bash
rtk init --codex --uninstall
```

## Exercise the integration

Run the optional test target from the repository root:

```bash
make test-rtk
```

It checks that RTK rewrites representative Git and Go test commands, runs the
existing Go suite through RTK, and prints RTK's project-scoped savings report.
The default `make test` and CI remain independent of RTK. A missing RTK binary
fails only the explicitly requested `test-rtk` target with an install hint.

## Context governance: the 3 rules of safe compression

Compressing terminal output saves context tokens, but over-compression risks creating an **information bottleneck** where the agent hallucinates a root-cause explanation due to missing evidence. Less context is not necessarily better context.

To govern context safely, Downshift follows three guiding rules:

1. **Safe squelch on success (low-risk output):**
   Automatically compress high-volume, low-risk commands when they succeed (e.g., file listings, clean `git status`, test runs passing with 0 failures). A single line summary like `Go test: 863 passed` preserves all actionable signal.
2. **Fail-open / raw preservation on error (high-risk output):**
   Preserve or make immediately retrievable the complete, uncompressed raw log whenever a command exits with a non-zero code, triggers a compilation error, or encounters a race condition/panic. Diagnosis requires full stack traces and context lines.
3. **Task risk-aware governance:**
   Tie output compression level to Downshift's task complexity and risk assessment. High-risk tasks (incident investigation, cryptographic code, security gates) prioritize fidelity and evidence completeness over aggressive token squelching.

## Native context compressor

Downshift includes an in-process, deterministic Go context compressor (`internal/compressor`) that requires no external binaries, Python, or network access:

- **Formats recognized:** `go test` (aggregates 2+ passing packages; fails-open on failures), `git status` (squelches clean working trees), `git log` (compacts commits to oneline), search results (`rg`/`grep` truncation after 20 matches), file lists (`ls -R`/`tree`), and repetitive logs (collapses identical consecutive lines).
- **Three operating modes:**
  - `off`: zero compression.
  - `observe` (default): calculates savings and detects format without modifying output bytes.
  - `safe`: applies deterministic compaction only when provably safe (0 errors).
- **CLI testing:**
  ```bash
  go test ./... | downshift context compress observe
  go test ./... | downshift context compress safe
  ```

## Harness capability matrix

> **Fundamental Principle:** Observability is not transformability. A hook that observes tool output (e.g. `PostToolUse`) cannot rewrite the model's context unless the harness exposes an explicit output-mutation channel.

| Harness | Events available | Observation capability | Context transformation capability | Subagent inheritance | Test evidence |
|---|---|---|---|---|---|
| **Claude Code** | `PreToolUse`, `PostToolUse`, `SubagentStop` | `PostToolUse` observes tool duration, status & resolved model | `PreToolUse` rewrites `updatedInput.model` (model only). Terminal output compaction runs via external proxy (`rtk`) or explicit pipe. | Inherited via subprocess environment | [claude-code-rewrite-honored](evidence/claude-code-rewrite-honored-2026-10-06.md) |
| **Codex** | `PreToolUse` (`multi_agent_v2`) | Hook payloads carry prompt & tools | Instruction-based (`RTK.md` / `AGENTS.md`) | Inferred from parent | [codex-rewrite-honored](evidence/codex-rewrite-honored-2026-10-02.md) |
| **Cursor** | `preToolUse` | Hook payload inspection | Model steering; instruction-based terminal filter | Discarded on Free/legacy Pro | Unconfirmed |
| **Antigravity** | `PreToolUse` | Hook payload inspection | Model steering; zero-dependency in-process compressor | Subagent inherits config | Labs validation |
| **KiroCrew** | `preToolUse`, `postToolUse` | Compliance observation | Policy mode (exit 0/2 only, no rewrite channel) | Direct spawn policy | In labs |

## Harness contract

| Concern | Guia / sensor | Type | Eixo |
|---|---|---|---|
| Use RTK only for supported commands; prefer raw output when completeness matters | `RTK.md` | Inferential guia | Behaviour |
| Context governance: squelch on success, preserve on error, respect task risk | This document | Inferential guia | Architecture fitness |
| Native compression: deterministic, in-process, fail-open on any error | `internal/compressor` | Computational sensor | Architecture fitness |
| Keep the optional test reproducible and fail on command/test errors | `scripts/test-rtk.sh` via `make test-rtk` | Computational sensor | Maintainability |
| Preserve independent Downshift and RTK hook responsibilities | This document | Inferential guia | Architecture fitness |



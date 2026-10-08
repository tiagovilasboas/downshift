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

## Harness contract

| Concern | Guia / sensor | Type | Eixo |
|---|---|---|---|
| Use RTK only for supported commands; prefer raw output when completeness matters | `RTK.md` | Inferential guia | Behaviour |
| Context governance: squelch on success, preserve on error, respect task risk | This document | Inferential guia | Architecture fitness |
| Keep the optional test reproducible and fail on command/test errors | `scripts/test-rtk.sh` via `make test-rtk` | Computational sensor | Maintainability |
| Preserve independent Downshift and RTK hook responsibilities | This document | Inferential guia | Architecture fitness |


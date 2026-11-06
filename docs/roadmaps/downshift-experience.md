# Downshift: installation confidence roadmap

Status: implementation in progress on a local branch; the complete repository check passed after the latest code changes. Host-session proof and a comparative quality/cost evaluation remain open. Estimates are Fibonacci story points, not durations. Each item is an independently reviewable increment; dependencies describe ordering, not implied completion.

## Baseline (6 November 2026)

On one Codex installation, the CLI and MCP status responded and the compactor ran a Git command. Doctor reported Codex wiring but no recorded hook event, and quota headroom was unknown. The CLI reported a resolved module version while MCP status reported `dev`. Doctor truncated remediation text. These observations are a single installation, not a representative benchmark. No before/after token cost or accepted-task quality data is available.

## Tasks

| Order | Task | Points | Dependency | Acceptance evidence |
|---|---|---:|---|---|
| 1 | Preserve complete doctor remediation commands with readable wrapping | 2 | None | Tests cover narrow widths, Unicode and complete command text; JSON stays unchanged. |
| 1 | Report the resolved running version consistently through MCP | 3 | None | MCP initialization and status match CLI resolution for stamped, module and development builds. |
| 2 | Filter doctor collection and output by harness | 3 | None | Valid and invalid IDs, global checks, JSON and exit status have documented tests. |
| 2 | Distinguish configured, connected and observed hook execution | 3 | None | Each state is based on explicit evidence; historical or synthetic events never claim current-session execution. |
| 3 | Add a guided integration self-test | 5 | State contract | Independently report compactor success and failure/exit code, MCP response, hook configuration and synthetic smoke; never claim host execution without host evidence. |
| 3 | Reduce repeated MCP tool descriptions | 3 | Representative baseline | Compare tokens and task outcomes on the same suite; retain essential delegation, safety and lifecycle guidance, including clients that defer tools. |
| 4 | Measure savings per accepted task | 5 | Baseline suite | Record sample size, task acceptance, critical failures, retries, latency and tokens; reject regressions rather than claiming savings from token counts alone. |

Tasks on the same files are serialized; otherwise order 1 and 2 can run in parallel. The MCP version and description tasks share files and must be serialized. PRs should remain individually mergeable. No release, deployment or production changes are implied.

## Guide and sensor

Guides: document evidence meanings and the supported installation workflow (inferential, behaviour); provide explicit MCP usage and fallback guidance (inferential, behaviour). Sensors: deterministic tests for rendering, version and protocol behaviour in local checks and CI (computational, behaviour); real host hook events and observed acceptance metrics after installation (computational, behaviour); review ambiguous diagnostics and semantic instruction coverage (inferential, maintainability/behaviour). Synthetic tests cannot establish that an actual host loaded its hooks.

## Progress

- Implemented: complete remediation text, resolved MCP version, harness filter, and isolated self-test. Hook wiring and historical events explicitly state their evidence limits.
- Pending: proof of actual host execution, representative MCP instruction evaluation and accepted-task cost baseline. The self-test does not establish token savings or host hook execution.

- MCP wire baseline: [one local observation](../evidence/mcp-instructions-baseline-2026-11-06.md); repeated server instructions were not present in individual tool descriptions on the wire. No comparable task outcomes were measured.
- Wire measurement script and Unicode test added; counts are repeatable, not evidence of task acceptance or token savings.
- Evaluation harness now has case prompts, synthetic fixture, isolated launcher and fail-closed comparator. One exploratory C03 pilot per arm ran; full paired independent scoring, native-host cases and token/cost data remain pending. Candidate instructions are not enabled.

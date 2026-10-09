# Harness x plan x rewrite-honored matrix

"Rewrite honored" means the harness executor applied the hook's model
rewrite to the child subagent, not just that downshift emitted one.
`outcome: "rewrite_emitted"` proves emission only (docs/session-models.md:17).

| harness | plan / build | rewrite honored? | evidence / limit |
|---|---|---|---|
| claude-code | paid: Pro / Max / Teams / API | Yes, 2026-10-06 (two spawns; API-reported model in the child transcript) | `updatedInput.model` must be a family name (`haiku`/`sonnet`/`opus`/`fable`); a full id is rejected and blocks the spawn. The catalog `native_name` supplies it. Write-up: `docs/evidence/claude-code-rewrite-honored-2026-10-06.md`. Needs `session-models.json` or the hook never rewrites. |
| claude-code | Free | No: no real subagents | Sandboxed, no egress; subagents need paid (README.md:728) |
| cursor | Free | No: silently discarded | `updated_input.model` ignored, known issue (README.md:734) |
| cursor | Pro legacy (request-based) | No: silently dropped | Task model takes only `"fast"` (README.md:735) |
| cursor | Pro / Ultra usage-based | Unconfirmed | Actual child-model proof remains P1.6. Separately, Cursor 3.24.9 blocks the external native quota collector before transport; see [diagnostic](evidence/cursor-gap-diagnostic.md). |
| codex | multi_agent_v2 + hooks on | Yes, inferred (2026-10-02) | Inferred, not observed: same-session follow-up: spawn requested `gpt-5.6-terra` after hook emitted Terra (`docs/evidence/codex-rewrite-honored-2026-10-02.md`). Payload >1MB still fail-open. Schema still evolving upstream. |
| kirocrew | any (`spawn_run` / `spawn_sub_agents`) | No: policy/block only | No `updated_input` channel; exit 0/2 only (README.md:399-404) |
| antigravity | desktop 2.21.1, `invoke_subagent` | Yes, observed 2026-10-09 | One exact child: original `pro`, executed `flash_lite`, native executor/generation model `gemini-3.5-flash-lite`. [Evidence and QA](evidence/antigravity-executor-ack.md). Manual read-only, build-specific sensor; quota/picker/alias mapping remains unresolved. |
| grok | any (single model grok-4.6) | No hook rewrite: config only | PreToolUse is allow/deny; pin `reasoning_effort` in config.toml (README.md:520-531) |

"Inferred" means a later spawn in the same session asked for the model the
hook had written (`rewrite_honored_inferred` in `stats --export`). It is
correlation, not a read of the child's actual model. Only applied rewrites
(outcome `rewrite_emitted`, no guardrail correction) count; allow, blocked
and held events never do.

## Revalidation

Every TBD cell must be re-checked per harness release: hook contracts
evolve upstream and a green cell can regress silently. Failing proof,
the tool fails open and the spawn runs unchanged (README.md:755).

A 2026-10-09 sample of Codex `spawn_agent` inputs recorded key sets only
(`agent_type`, `message`, `task_name`, and sometimes `fork_turns`). It did
not include hook stdin, `transcript_path`, a model field, or executor
acknowledgement, so the Codex row above is not refreshed. See
[closeout-recheck-2026-10-09.md](evidence/closeout-recheck-2026-10-09.md).

## Sources (README/docs only, no vendor claims)

- Claude paid: emission only until P1.3 (docs/evidence/rewrite-honored-protocol.md); plan limits: README § Plan compatibility
- Cursor free/legacy discard: README.md:734-735, README.md:738, README.md:1012-1013
- Codex flags + honored: README.md:744, README.md:746; evidence 2026-10-02: docs/evidence/codex-rewrite-honored-2026-10-02.md; evolving schema: README.md:373
- KiroCrew binary contract: README.md:399-404; support row: README.md:692
- Antigravity observed child, dated build and evidence limits: [native evidence](evidence/antigravity-executor-ack.md) and [final QA](evidence/final-gap-qa.md)
- Grok config-only: README.md:520-531, README.md:691, README.md:1236
- Emission vs honored: docs/session-models.md:17; allowlist: docs/session-models.md:34

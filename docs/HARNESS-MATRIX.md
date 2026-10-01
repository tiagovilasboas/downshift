# Harness x plan x rewrite-honored matrix

"Rewrite honored" means the harness executor applied the hook's model
rewrite to the child subagent, not just that downshift emitted one.
`outcome: "rewrite_emitted"` proves emission only (docs/session-models.md:17).

| harness | plan / build | rewrite honored? | evidence / limit |
|---|---|---|---|
| claude-code | paid: Pro / Max / Teams / API | Yes | PreToolUse + updatedInput honored (README.md:725) |
| claude-code | Free | No: no real subagents | Sandboxed, no egress; subagents need paid (README.md:728) |
| cursor | Free | No: silently discarded | `updated_input.model` ignored, known issue (README.md:734) |
| cursor | Pro legacy (request-based) | No: silently dropped | Task model takes only `"fast"` (README.md:735) |
| cursor | Pro / Ultra usage-based | Yes (TBD) | Works where selection expanded (README.md:738); TBD: re-check per Cursor release |
| codex | multi_agent_v2 + hooks on | Yes (TBD) | Model + reasoning_effort honored (README.md:744); TBD: v2 schema still evolving (README.md:373) |
| kirocrew | any (`spawn_run` / `spawn_sub_agents`) | No: policy/block only | No `updated_input` channel; exit 0/2 only (README.md:399-404) |
| antigravity | `invoke_subagent` build | Gated: rewrites only with native catalog entries (pending — none in catalog.json yet, so R4 holds every decision and the adapter observes fail-open) | Session-gated alias flash/flash_lite (README.md:379-384); TBD: re-check alias set per build |
| grok | any (single model grok-4.6) | No hook rewrite: config only | PreToolUse is allow/deny; pin `reasoning_effort` in config.toml (README.md:520-531) |

## Revalidation

Every TBD cell must be re-checked per harness release: hook contracts
evolve upstream and a green cell can regress silently. Failing proof,
the tool fails open and the spawn runs unchanged (README.md:755).

## Sources (README/docs only, no vendor claims)

- Claude paid honored: README.md:725; free limits: README.md:728, README.md:1230
- Cursor free/legacy discard: README.md:734-735, README.md:738, README.md:1012-1013
- Codex flags + honored: README.md:744, README.md:746; evolving schema: README.md:373
- KiroCrew binary contract: README.md:399-404; support row: README.md:692
- Antigravity overwrite: README.md:379; support row: README.md:690
- Grok config-only: README.md:520-531, README.md:691, README.md:1236
- Emission vs honored: docs/session-models.md:17; allowlist: docs/session-models.md:34

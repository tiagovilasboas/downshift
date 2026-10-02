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
| codex | multi_agent_v2 + hooks on | Yes (2026-10-02) | Same-session follow-up: spawn requested `gpt-5.6-terra` after hook emitted Terra (`docs/evidence/codex-rewrite-honored-2026-10-02.md`). Payload >1MB still fail-open. Schema still evolving upstream. |
| kirocrew | any (`spawn_run` / `spawn_sub_agents`) | No: policy/block only | No `updated_input` channel; exit 0/2 only (README.md:399-404) |
| antigravity | `invoke_subagent` build | Gated: Yes when session lists `flash_lite` / `flash` / `pro` and guardrails clean (catalog shipped 2026-10-01) | Session-gated aliases; R4 no longer blocks native Gemini tiers; TBD: honor rewrite on real spawn (P5.5) |
| grok | any (single model grok-4.6) | No hook rewrite: config only | PreToolUse is allow/deny; pin `reasoning_effort` in config.toml (README.md:520-531) |

## Revalidation

Every TBD cell must be re-checked per harness release: hook contracts
evolve upstream and a green cell can regress silently. Failing proof,
the tool fails open and the spawn runs unchanged (README.md:755).

## Sources (README/docs only, no vendor claims)

- Claude paid honored: README.md:725; free limits: README.md:728, README.md:1230
- Cursor free/legacy discard: README.md:734-735, README.md:738, README.md:1012-1013
- Codex flags + honored: README.md:744, README.md:746; evidence 2026-10-02: docs/evidence/codex-rewrite-honored-2026-10-02.md; evolving schema: README.md:373
- KiroCrew binary contract: README.md:399-404; support row: README.md:692
- Antigravity overwrite: README.md:379; support row: README.md:690
- Grok config-only: README.md:520-531, README.md:691, README.md:1236
- Emission vs honored: docs/session-models.md:17; allowlist: docs/session-models.md:34

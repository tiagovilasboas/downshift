# Vendor hook documentation vs our adapters (2026-10-11)

Source: web search summaries of the official pages. The vendor hosts were not
fetchable from the build container, so these are **summaries of vendor pages,
not page captures**. Re-check against the live page before relying on a field.
Forum and third-party items are marked and carry no weight as contract.

| Harness | Documented input (vendor) | Documented output (vendor) | Difference from our adapter |
|---|---|---|---|
| Cursor ([hooks](https://cursor.com/docs/hooks)) | `tool_name`, `tool_input`, `tool_use_id`, `cwd`, model fields, `agent_message` | `permission` allow/deny (`ask` accepted but not enforced for preToolUse), `user_message`, `agent_message`, `updated_input`. Accepts Claude-style nested names. Invalid JSON blocks the action | We also read `workspace_roots` and `model_id`: confirm both on the page. Forum (unverified): `updated_input` was ignored for `Task` (reported fixed later) and for Write |
| Codex ([hooks](https://developers.openai.com/codex/hooks)) | common: `transcript_path` (not a stable interface); PreToolUse: `turn_id`, `tool_name`, `tool_input`, `tool_use_id`. Third-party captures add `session_id`, `cwd`, `permission_mode` | `hookSpecificOutput` with `hookEventName`, `permissionDecision`, reason. `updatedInput` accepted only with `allow`; Bash/apply_patch need a string `command`; MCP and other function tools take the replacement args object | We do not read `tool_use_id` or `turn_id` (not needed today). A bare `allow` without `updatedInput` is rejected by design; check that our no-rewrite path never emits it |
| Kiro ([hooks](https://kiro.dev/docs/hooks/types/)) | PreToolUse example: `hook_event_name`, `cwd`, `session_id`, `tool_name`, `tool_input`. Exit code 2 blocks; PostToolUse cannot block. 3.0 uses PascalCase event names, old camelCase maps to them | n/a (we do not rewrite on Kiro) | We do not read `session_id`. PostToolUse schema (including `tool_response`) **not confirmed** in these results |
| Antigravity ([hooks](https://antigravity.google/docs/hooks/)) | camelCase: `toolCall{name,args}`, `stepIdx`, `conversationId`, `workspacePaths`, `transcriptPath`, `artifactDirectoryPath`, `modelName` | `decision` required: allow, deny, ask, force_ask, deny_unless_prior_grant; optional `reason`, `permissionOverrides` | **`overwrite` is not in the summary of the documented output.** Our rewrite on Antigravity rests on `antigravity-executor-ack.md`, not on a documented field: re-verify. We do not read `modelName`, which could replace a model guess. Docs sample is inconsistent per the changelog |

## Follow-ups (each needs the live page or a real payload)
1. Antigravity: confirm whether any documented output field rewrites tool args; if none, mark the rewrite as unsupported-by-docs in `harness-matrix.md`.
2. Cursor: confirm `model_id` and `workspace_roots` on the page; re-test `updated_input` on `Task` on the current build.
3. Kiro: capture a real PostToolUse payload (see `kirocrew-native-hook-2026-10-10.md`).
4. Codex: confirm the no-rewrite path never emits a bare `allow`.

## Antigravity follow-up (search summary, same day)
- The JSON hook output documents no args-rewrite field. Our `overwrite` carries a `Subagents` array (`internal/adapters/antigravity/antigravity_test.go`), a different shape from any documented field.
- The Python SDK changelog mentions pre-tool hooks that can transform tool input via `modified_args`. That is the SDK, not the JSON hook, so it does not confirm our field.
- Status: the rewrite is **observed once** (`antigravity-executor-ack.md`), **not documented**. Keep the existing "build-specific sensor" wording; do not describe it as a supported contract.

## Community reports (search, unverified, not contract)
- Cursor: a live preToolUse payload on 3.13.x carried `model`, `cwd`, `workspace_roots`, `conversation_id`, `generation_id`, `hook_event_name`. `model_id` was not seen anywhere; keep parsing both. Top-level `cwd` can be empty with the real directory only in `workspace_roots` (already our fallback). Hooks have run with **empty stdin** on remote Linux workspaces: covered by `TestRunHookAdapter_EmptyStdinFailsOpen`.
- Claude Code: several issues (2026-03 to 2026-06) report `updatedInput` ignored for the Agent tool on some versions, and `CLAUDE_CODE_SUBAGENT_MODEL` working as a full override. Matches our `doctor` warning; the rewrite is honored per `claude-code-rewrite-honored-2026-10-06.md`, version-bound.
- Codex: no source found that settles whether a bare `allow` (no `updatedInput`) is rejected. Still needs a live test.

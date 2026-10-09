# Close-out recheck — 2026-10-09

harness-downshift by Tiago de Carvalho Vilas Boas
https://github.com/tiagovilasboas/downshift

Afternoon recheck after `739ecec`. This records what was observed and what that
observation does **not** close. No prompts, transcript paths, account
identifiers, token totals, or dollar amounts are stored here. The installed
hook binary was not replaced.

## What was checked

| Check | Result | Closes a gap? |
|---|---|---|
| Cursor `--version` | Still 3.24.9 (`cd6d2a1f2e56e9841f0ed9c7c24542087b4e69b0`) | No. Same-day repeat of the existing diagnostic does not close GAP-CURSOR-REVALIDATE / P5.3. |
| Codex CLI | `codex-cli 0.144.5` | No, by itself. |
| Hook command | Absolute `downshift codex` on PreToolUse matcher `(^Agent$\|.*spawn_agent$)` | Identifies the binary under test. It is not a payload. |
| That binary | Version string `dev`. `downshift quota` exits 2 with `unknown command "quota"`. Byte search finds `agent_transcript_path` once and no separate `transcript_path`, `claude-statusline`, `quota collect`, or `DOWNSHIFT_CURSOR_NATIVE_FILE`. | No. The process the hook actually runs cannot collect Codex quota or serve the Claude statusline bridge. |
| Codex spawn inputs | 15 `spawn_agent` inputs in session files modified within 72 hours. Key sets: 9 were `agent_type`, `message`, `task_name`; 6 also included `fork_turns`. No `model` or `reasoning_effort` key in those inputs. | Spawn-behavior note only. Does not close GAP-CODEX-REVALIDATE / P5.4. |
| Hook stdin | 0 JSON objects with a `hook_event_name` key. 0 JSON keys named `transcript_path`. The Codex binary's schema strings do contain `transcript_path`. | Schema strings and session text are not a hook payload. GAP-CODEX-HOOK stays pending. |
| Claude settings | No `statusLine` member. No `rate_limits` key in scanned Claude settings JSON (session transcripts were not treated as a quota cache). No `quota.json` in the Downshift state directory. | GAP-CLAUDE-QUOTA stays pending. An empty statusline was not wired, because the installed binary cannot run the bridge and no live consumption was observed. |
| Usage volume | Not re-audited. No paid calls were added. | GAP-USAGE-50, GAP-CONTROL, and GAP-PERIOD-REPORT stay pending. |

`codex features list` failed closed on the local config (`agents` did not parse
as the expected role struct). Feature-flag state was not read. That failure is
not a hook schema sample.

Recent session files do contain native `token_count` events, so a future
outside-hook collector has a source. This recheck did not connect one: the
binary the hook runs cannot consume it, and replacing that shared binary would
also change the Claude Code and Cursor hook commands. No Cursor export exists
for that binary to consume.

## Acceptance result

No runtime gap moved to done. QA verdict: PASS in
[closeout-recheck-qa.md](closeout-recheck-qa.md).

GAP-GRAPHIFY / P4.8 is **deferred**, not done. It stays optional and out of
this close-out. The existing command fetcher is unchanged.

Beta remains on. Pillar exits are unchanged.

Guia: acceptance text in [gap-tasks.md](../gap-tasks.md). Sensor: this dated
negative recheck (behaviour). It is an evidence record, not a new CI gate.

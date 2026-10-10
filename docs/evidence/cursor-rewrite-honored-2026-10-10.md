# Cursor rewrite honor — 2026-10-10

harness-downshift by Tiago de Carvalho Vilas Boas
https://github.com/tiagovilasboas/downshift

One spawn on this machine, Cursor **3.24.9** build
`cd6d2a1f2e56e9841f0ed9c7c24542087b4e69b0`. The hook binary was
`/Users/tiago.boas/.local/bin/downshift` at `v0.1.0-beta.7-69-g379d413`.
No prompts, transcript paths, account identifiers, or quota payloads are
stored here.

## Spawn

| Field | Observed |
|---|---|
| Requested model | `composer-2.5` |
| Hook outcome | `rewrite_emitted` |
| Verdict | `UPSHIFT` |
| Final model written by the hook | `muse-spark-1.3-max` |
| Event time | `2026-10-10T05:10:05.433107Z` |
| `rewrite_honored` | absent |
| Child executor model | not present in the subagent transcript |
| Model slug in the child instructions | unstated |

The child process finished. Finishing is not an acknowledgement of
`muse-spark-1.3-max`. The transcript has no native executor model field.
P1.6 and GAP-CURSOR-HONOR stay open.

## Stopped items

| Item | Result |
|---|---|
| GAP-CLAUDE-QUOTA | `~/.claude/settings.json` has no `statusLine`. No `rate_limits` payload was supplied. Stays pending. |
| Slice 2 | No new documented hook field for the executed child model or that child's tokens on Cursor, Codex, Antigravity, Kirocrew, or Grok. No port entry added. |
| P3.4 / P3.7 | `downshift stats --days=7 --export` reported `usage_records` 15, `usage_linked` 5, `baseline_no_route` 0. Count was not filled to 50. No real control period was present. |
| P2.3 | No operator search. Stays open. |

Beta exit was not declared.

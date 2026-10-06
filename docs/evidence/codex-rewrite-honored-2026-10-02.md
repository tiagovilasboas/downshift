# Codex rewrite honored — 2026-10-02

**P1.5** in `docs/beta-exit.md`. Emission vs honored: `docs/session-models.md`.

## Setup

- Harness: Codex UI, `multi_agent_v2` + hooks on
- Downshift: local binary `dev` on this machine, after the confidence gate
- Session (hashed in `events.jsonl`): `0166fc679c058aeff3af3772d8a228b5f00442372de9c1ba7d4f6015a2c7cd38`

## What happened

1. Spawn arrived as `gpt-6-luna`. Downshift classified `MEDIUM` and emitted `gpt-5.6-terra` (`outcome: rewrite_emitted`, verdict `UPSHIFT`).
2. The next spawn on the **same hashed session** arrived with `requested_model: gpt-5.6-terra` and stayed on Terra (`verdict: OK`).

That follow-up is the honor signal Codex exposes today. It is an **inference**: a later spawn asked for the model the hook wrote. Downshift does not read the model the child actually ran on, so this is correlation, not direct observation.

`PAYLOAD_TOO_LARGE` still happens when the Codex UI sends the whole thread over 1 MB. Those events are fail-open, not a failed rewrite.

## What this is not

- Not Anthropic/OpenAI billed tokens (`real_cost_events` still 0).
- Not Claude Code Task on a paid plan (P1.3).
- Not a claim that every Codex build will keep this schema.

## Stats

`downshift stats --export` now reports `rewrite_shifted` and `rewrite_honored_inferred` from this same-session follow-up pattern. It does not rewrite `events.jsonl`.

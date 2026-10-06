# Claude Code rewrite honored — 2026-10-06

**P1.3** in `docs/BETA-EXIT.md`. Emission vs honored: `docs/session-models.md`.

## Setup

- Harness: Claude Code (desktop app), `Agent` tool, PreToolUse hook on `Task|Agent`
- Parent session model: Sonnet 5.5
- Session (hashed in `events.jsonl`): `ac0d9bb4af86d7b2409f055d0d1e04db482a07c655ef677e8275842ed95441d7`
- Plan tier: not recorded

## What happened

Same prompt twice: *rename the userId variable to userIdentifier in auth.ts* (no `auth.ts` exists in the repo; the task only needs to classify).

1. **08:59Z, build `local-4ba5a71`: rewrite rejected, spawn blocked.** The hook classified `TRIVIAL` and wrote `claude-haiku-4-5` into `updatedInput.model`. The harness validates that field against a fixed enum of family names and refused it:

   ```
   PreToolUse hook for Agent returned updatedInput that failed schema validation:
   model: expected one of "sonnet" | "opus" | "haiku" | "fable"
   ```

   The event was still logged as `rewrite_emitted`. Emission is not honor, and a rejected rewrite here fails the spawn instead of running it unchanged.
2. **Fix `9476873`:** the catalog now carries a family-level `native_name`; the Claude Code adapter writes it instead of the full id and allows the spawn unchanged when an entry has none.
3. **09:06Z, build `local-9476873`: rewrite accepted.** The hook wrote `haiku`. The spawn ran and the subagent reported it was running as `claude-haiku-4-5-20251001`, while the parent session is Sonnet 5.5, so the child did not inherit the session model.

## Strength of the evidence

- The model name comes from the subagent's **self-report**. It is consistent with the rewrite and differs from the parent, but it is not a billing or usage record.
- One session, one spawn after the fix. Not a rate.
- Downshift still does not read the model the child ran on (see the Codex write-up).

## What this is not

- Not Anthropic billed tokens (`real_cost_events` still 0): cross-check with P3.4/P3.5.
- Not a statement about other harnesses. Cursor, Codex, Antigravity and KiroCrew still write the catalog id; none was seen rejecting it, none was re-tested for this.

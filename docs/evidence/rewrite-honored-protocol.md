# Manual protocol — rewrite honored (P1.3 / P1.5 / P1.6)

One spawn is enough. Do not paste prompts into issues.

1. Install the current `downshift` binary and confirm hooks for that harness.
2. Start a **new** session so the payload stays under 1 MB.
3. Ask the parent to spawn **one** subagent with a trivial task (example: reply with the word `pong`).
4. Note the child model in the UI, if the harness shows it.
5. Run `tail -n 20 ~/.harness-downshift/events.jsonl` and look for the same harness:
   - first line: `requested_model` ≠ `final_model`, `outcome: rewrite_emitted`
   - later line, same `session_id` (hashed): `requested_model` equals the previous `final_model`
6. Paste only `downshift stats --export` (no JSONL lines with models you consider sensitive) into the evidence note.

Claude Code paid: this is P1.3. Codex `multi_agent_v2`: P1.5 (done 2026-10-02). Cursor Pro/Ultra usage-based: P1.6.

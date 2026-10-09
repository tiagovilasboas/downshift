# Close-out recheck QA — 2026-10-09

Independent check of the honesty recheck against main `739ecec`. Tests and
evidence only. No production edit, no commit, no push, no installed-binary
replacement, no settings edit, and no paid model turn. Prompts, transcript
paths, account identifiers, token totals, and dollar amounts are omitted.

Verdict: PASS

The recheck docs do not mark a runtime gap done. The machine facts below match
the claims that were checked.

## Checked facts

| Fact | Result |
|---|---|
| Installed `downshift quota` exits non-zero and stderr contains `unknown command`. The Codex hook command is that absolute binary, with argument `codex`, on the PreToolUse `spawn_agent` matcher. | PASS. Exit code 2. Stderr contains `unknown command`. Hook command is the absolute installed binary. |
| `cursor --version` first line is 3.24.9. | PASS. First line is `3.24.9`. |
| `codex --version` reports 0.144.5. | PASS. Output is `codex-cli 0.144.5`. |
| Closure checklist in `docs/gap-tasks.md`: no gap checkbox newly marked done versus `739ecec`. GAP-GRAPHIFY stays unchecked and is labeled deferred, not done. | PASS. Diff against `739ecec` shows no checkbox moving to done. GAP-GRAPHIFY remains `[ ]`, with the words deferred and not done. |
| `docs/beta-exit.md`: release item "All five Exit lines above satisfied" stays unchecked. P1.6, P2.3, P2.4, P3.4, P5.3, and P5.4 stay open. P4.8 is not checked done. | PASS. Those items are `[ ]` in the working tree and were not done at `739ecec`. P4.8 is unchecked and labeled deferred, not done. |
| Codex matrix row stays inferred 2026-10-02, not a 2026-10-09 honor refresh. | PASS. The Codex row is unchanged from `739ecec` and still says `Yes, inferred (2026-10-02)`. The 2026-10-09 note says the row was not refreshed. |
| Spawn-input key sets in Codex session files modified within 72 hours: 15 inputs; 9 with keys `agent_type`, `message`, `task_name`; 6 that also include `fork_turns`; no `model` or `reasoning_effort` key. | PASS. Independent count matches 15 / 9 / 6. Neither `model` nor `reasoning_effort` is present on those inputs. Key values were not recorded. |

## Doc claim check

`docs/evidence/closeout-recheck-2026-10-09.md` states that no runtime gap moved
to done, that GAP-GRAPHIFY / P4.8 is deferred, and that beta stays on.
`docs/gap-tasks.md` leaves the quota, hook, usage-volume, Cursor revalidation,
and Codex revalidation tasks pending. `docs/beta-exit.md` leaves the release
checklist open. `docs/harness-matrix.md` does not move the Codex honor cell to
2026-10-09.

## Residual limits

This pass does not close GAP-CODEX-HOOK, GAP-CLAUDE-QUOTA, GAP-CURSOR-REVALIDATE,
GAP-CODEX-REVALIDATE, or beta exit. The spawn count is a key-set spot check, not
hook-stdin proof and not executor acknowledgement. The Cursor build hash in the
recheck note was not re-read; only the version first line was checked.

<!-- Copyright (c) 2026 Tiago de Carvalho Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Next Steps — Roadmap to v0.1.0-rc.1

**Current state:** Beta 5 with P1 complete (Claude Code rewrite honored), P3 hooks ready for data collection, infrastructure solid.

**Goal:** Exit beta when ≥3 of 5 pillars have evidence.

---

## Immediate (This Week)

### 1. Collect Real Usage Data (P3.4 + P3.5)

**What's running now:**
- `downshift claude-code` (PreToolUse): rewrites model in real time ✓
- `downshift claude-code-subagent-stop` (SubagentStop): prices subagents from their transcript ✓

**Action:**
- Spawn 3-5 trivial tasks in Claude Code (they will downshift to haiku).
- Spawn 3-5 complex tasks (they will upshift to opus).
- Let subagents complete. The SubagentStop hook will record real tokens and cost.
- After ~50 usage events across a few days, export: `downshift stats --days=7 --export > my-stats.json`

**Blockers to watch:**
- Savings show $0 if the PreToolUse event has `requested_model: "unknown"` (the spawn didn't pass a model field). The code handles this; it's expected until harnesses pass explicit models.

---

### 2. Recruit External Users (P2.3 + P2.4)

**What's needed:**
- ≥2 teammates or external users to run the hooks for 1 week.
- They each export: `downshift stats --days=7 --export > their-stats.json`
- They paste into a GitHub issue (or email) — no prompts, redact paths if paranoid.

**Instructions to give them:**
See [docs/session-models.md](session-models.md#installation) for fresh installation and hook setup.

**Exit criteria:**
- ≥3 distinct session sources aggregated in README "Real session data" table with dates.

---

## Medium-term (Week 2–3)

### 3. Verify P4 — Blind Dataset

**What's blocked:**
- The holdout dataset is burned (tuned to 100%).
- The seed dataset shows 69% accuracy.
- Need a new blind dataset (200+ tasks, no tuning leakage).

**Action:**
- Either: (a) collect tasks from external users + label blind, or (b) find a public dataset of coding tasks.
- Run `downshift benchmark blind-dataset.json --gate` with a reasonable accuracy floor (60-70%).

---

## Later (Before RC)

### 4. Cursor P1.6 — Rewrite Honor Proof

Repeat the Claude Code P1.3 protocol on Cursor Pro/Ultra:
- Wire the hooks.
- Spawn a trivial task.
- Capture proof that the child ran on the downshifted model.
- Document in `docs/evidence/cursor-rewrite-honored-YYYY-MM-DD.md`.
- Update [HARNESS-MATRIX.md](HARNESS-MATRIX.md) row for Cursor.

---

### 5. Other Harnesses (P1.8+)

- **Codex**: P1.5 already done (2026-10-02).
- **Antigravity, KiroCrew**: adapt the protocol. Antigravity needs `flash_lite` / `flash` / `pro` mapping; KiroCrew is policy-only (no rewrite in the hook).

---

## Release Checklist

✅ = Complete | ⏳ = In progress | ⚪ = Blocked

| Item | Status | Notes |
|------|--------|-------|
| P1.3: Claude Code honor proof | ✅ | [docs/evidence/claude-code-rewrite-honored-2026-10-06.md](evidence/claude-code-rewrite-honored-2026-10-06.md) |
| P3.2–P3.3: PostToolUse hooks | ✅ | Both PreToolUse and SubagentStop in place |
| P3.4: ≥50 usage events | ⏳ | Collect real data (you, this week) |
| P3.5: Real cost in stats | ⏳ | Depends on P3.4 data |
| P2.3: 2+ external operators | ⏳ | Recruit teammates |
| P2.4: README multi-source note | ⏳ | Depends on P2.3 |
| P4: Blind dataset | ⚪ | Depends on external data |
| P1.6: Cursor proof | ⚪ | Next harness |
| v0.1.0-rc.1 tag | ⚪ | When ≥3 pillars done |

---

## Troubleshooting

**"SubagentStop isn't firing"**
- Confirm your `~/.claude/settings.json` has the SubagentStop hook. See [session-models.md](session-models.md#claude-code-real-usage).
- The hook is silent (fail-open) — check `~/.harness-downshift/events.jsonl` for `"outcome": "usage"` records.

**"Savings still show $0"**
- If `requested_model` is `unknown`, the spend is priced but savings are zero (no baseline to compare against).
- This is correct behavior — the hook can only save when it knows what the spawn would have requested.

**"I want to skip P2/P4 and go straight to RC"**
- That's a business call. The exit criteria are in `docs/BETA-EXIT.md` — change them if you want to.

---

**Questions?** See [docs/BETA-EXIT.md](BETA-EXIT.md) for the full pillar breakdown, or open an issue.

# Public beta — honesty, blockers, and how to help

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

The **beta** label stays until [beta-exit.md](../reference/beta-exit.md) is signed.
This page is what an external developer can verify locally, and what still needs
a real operator. It is not a release announcement.

## What you can trust today

| Surface | Claim that matches the code |
|---------|-----------------------------|
| Claude Code | Primary path. PreToolUse can rewrite a subagent model. Honor evidence exists for one paid session (2026-10-06) via the child transcript model, not an invoice. `SubagentStop` records transcript token counts and model; current Downshift reports show no dollar amount (`internal/adapters/claudecode/subagentstop_test.go`). |
| Codex | Functional wire path. Honor write-up is inferred (2026-10-02), not a billing receipt. |
| Cursor | Functional wire path. **Rewrite honored is not closed** (beta-exit P1.6). Do not treat a hook JSON rewrite as proof the child ran that model. |
| KiroCrew, Antigravity | Experimental. Matrix rows stay honest when a plan drops the rewrite. |
| Classifier | Deterministic regex (EN + PT) plus an in-process FNV hash boost. No Graphify, no E5, no external MiniLM command. |
| Usage reporting | `stats`, `report` and `stats --export` report observed tokens; they show no dollar amount. The 10% control group lets `report` compare routed and unrouted tokens per completed task. It is not a billing comparison. |

Local checks that do not spend provider credit:

```bash
go test ./...
go run ./cmd/downshift try "rename userId to accountId" claude-code
go run ./cmd/downshift doctor
```

Synthetic usage fixtures prove the **recorder**. They do not close pillar P3.

## Blockers that still keep the beta label

Copied from beta-exit (2026-10-15), still open after the 2026-10-17 cleanup:

| ID | Blocker | Why this repo cannot close it alone |
|----|---------|--------------------------------------|
| P1.6 | Cursor rewrite honored | Needs a paid Cursor session and a matrix row. Not run here. |
| P2.3–P2.4 | Two or more non-maintainer `stats --export` | Needs other operators for a week. |
| P3.4 | ≥50 `usage` events linked to PreToolUse | Needs real Claude Code SubagentStop traffic. Fixtures are not that count. |
| P3.7 | One calendar week vs the provider dashboard | Needs the [billing comparison template](billing-comparison-template.md) filled by the operator. |
| P4 | Blind set ≥200 tasks never used to tune signals | Public `benchmark/report.md` splits are not that set. Do not tune on them. |
| P5.3–P5.4 | Quarterly Cursor and Codex revalidation | Dogfood, not a code change. |

### Protocol for a future P3.4 run (do not treat as done)

1. Install hooks with `downshift setup` on **Claude Code only**, including SubagentStop (`docs/guides/install.md`).
2. Use the product for real work until `downshift stats --export` shows `usage_linked` ≥ 50.
3. Fill one week in the billing template. Keep the 10% control group on.
4. Paste the export with prompts and paths removed ([stats-export.md](../reference/stats-export.md)).

## Roadmap for contributors

**Now**

- Misroute issues with `downshift try` output (EN or PT).
- Docs that still describe a removed flag (Graphify, `DOWNSHIFT_E5_*`, `DOWNSHIFT_MINILM_EMBED`).
- Adapter tests that use fixtures already in `examples/`.

**Next**

- Anonymised exports from a second machine (P2).
- A blind prompt list of 200+ tasks stored outside the signal-tuning loop (P4). Do not land signal changes in the same PR.

**Later**

- Cursor honor evidence (P1.6) and any harness usage API the vendor actually exposes (P3.8).
- Promoting shadow paths (NB, scores, adapt memory) only after an independent eval.

## Proposed issues (not opened)

Open these only if a search shows no duplicate:

1. `good first issue` — `misroute:` template walkthrough in CONTRIBUTING is enough; add one fixture prompt that is currently Medium with no signal and must stay Medium.
2. `help wanted` — collect two redacted `stats --export` JSON files from non-maintainers (P2.3).
3. `help wanted` — draft 200 blind prompts (id, prompt, expected tier) in a gist or labs repo, with a written rule that `signals.go` must not change in that PR.

## LinkedIn draft (do not post until the maintainer says so)

Downshift is a local hook that picks a smaller model for a coding-agent subtask using deterministic rules (English and Portuguese) and an in-process hash, not an LLM in the loop. It is still **beta**: Claude Code is the path with honor evidence; Cursor honor and real billing comparisons are open. If you run it for a week, `downshift stats --export` (counts only, no prompts) is the most useful thing you can send. Apache-2.0. https://github.com/tiagovilasboas/downshift

## Cost of this readiness pass

Documentation and local `go test` only. Incremental provider spend: none.

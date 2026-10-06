# Engineering loop — maintainer appendix

User-facing summary: [ENGINEERING-LOOP.md](../ENGINEERING-LOOP.md).  
Not required for install; contains promotion thresholds and historical eval notes.

---

## dsmon (live tail)

`cmd/dsmon` tails `events.jsonl` and shows recent verdicts and rough savings
estimates. Build: `go build -o dsmon ./cmd/dsmon`. Savings lines are directional,
not provider billing unless usage fields are present on events.

---

## Promotion loop (offline)

1. Collect prompt-free routing metadata locally.
2. Review tasks; `downshift feedback` with explicit `--required-tier` when needed.
3. `downshift train --from-events` → candidate weights.
4. `downshift benchmark <file> --compare --candidate-weights=...` on sets **not**
   used to edit `signals.go`. `benchmark/holdout.json` is burned for tuning.
5. v2 is **not** promoted to the hook from this loop without a separate decision,
   fresh eval set, and rollback plan.

No online learning from a single hook invocation. Unreviewed success does not
imply a minimum tier label.

---

## Historical v2 comparison (do not cite as current gate)

On an older small seed (not the 500-task split), candidate v2 underperformed
legacy on tier accuracy with higher unsafe downgrades. v2 stays CLI/shadow only
until a fresh comparison on an untouched set passes agreed safety gates.

---

## Remaining closed-loop work

- Harness completion/retry signals per stable vendor contracts.
- Per-harness outcome rate reporting with sample-size honesty.
- CI contract checks for preservation and fail-open across adapters.
- Explicit promotion threshold for unsafe downgrades before any hook switch.

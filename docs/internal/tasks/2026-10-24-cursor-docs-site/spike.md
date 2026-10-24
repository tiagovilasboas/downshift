# Spike: docs and site after Cursor composer baseline (2026-10-24)

Downshift by Tiago Vilas Boas  
https://github.com/tiagovilasboas/downshift

## Problem

Code and reference matrices (`harness-matrix`, `capability-matrix`) describe the
3.27.12 probe, `cursor-touch`, and `cursor-composer-model.json`, but user-facing
install text, README, `config.md`, and the public site still read like “Task
`preToolUse` only” with no baseline story. That mismatch confuses setup and
support.

## Inventory (what must say the same thing)

| Surface | Repo | Cursor gap before this batch |
|---|---|---|
| Install (Cursor) manual JSON | downshift `docs/guides/install.md` | Missing `sessionStart`, `beforeSubmitPrompt`, `subagentStop`; no `setup cursor` as primary path |
| State files | downshift `docs/guides/config.md` | No `cursor-composer-model.json` |
| README harness blurb | downshift `README.md` | No composer cache / R9 when baseline empty |
| `llms.txt` | downshift | No `cursor-touch` |
| Changelog | downshift `CHANGELOG.md` | Unreleased silent on Cursor adapter |
| Task pack cross-link | downshift `docs/internal/tasks/2026-10-24-cursor-live/` | Docs/site called “optional” |
| Harness table (home) | downshift-site `index.html`, `pt-br/index.html` | Cursor row omits child-end verification date |
| Harness table (Next) | downshift-site `apps/site/.../section-harnesses.tsx` | Same |
| Mirrored docs HTML | downshift-site `docs/**/index.html` | Copy of old `install.md` (no auto-sync script in repo) |
| Roadmap RL-5 | downshift `docs/internal/tasks/2026-10-24-roadmap/` | Partially satisfied by harness copy updates |

## Out of scope (explicit)

- Claiming rewrite **honored** on Cursor (still P1.6).
- Regenerating every mirrored matrix HTML page on the site (link to GitHub
  reference until a doc-sync pipeline exists).
- `gen-cursor-rules.sh --apply` (harness-core).
- Live proof of `rewrite_emitted` after cache warm (runtime evidence task).

## Sensor

After merge: `grep -r cursor-touch docs/guides/install.md README.md` and site
harness table mentions “3.27.12” or “child end verified”.

# Cursor docs and site sync (2026-10-24)

Downshift by Tiago Vilas Boas  
https://github.com/tiagovilasboas/downshift

Follows code in [2026-10-24-cursor-live](../2026-10-24-cursor-live/README.md) (merged PR #547).

| # | Pts | Task | Closes when |
|---|-----|------|-------------|
| D1 | 2 | Spike inventory | [spike.md](spike.md) |
| D2 | 3 | `install.md`, `config.md`, README, `llms.txt`, CHANGELOG, troubleshooting (R9 + cache) | Manual JSON matches `setup_targets`; state file documented |
| D3 | 1 | `TestSetupCursorTouchHooks` in `setup_test.go` | Asserts `sessionStart` and `beforeSubmitPrompt` wire `cursor-touch` |
| D4 | 2 | downshift-site harness table (static `index.html` + `section-harnesses.tsx`) | Cursor row cites 3.27.12 child end + honor still unconfirmed |
| D5 | 3 | Site `docs/install` HTML refresh | Cursor section aligned with D2 (EN; PT if same block exists) |
| D6 | 1 | Roadmap / cursor-live README cross-links | Points here for public copy |

D5 may lag D2 if HTML is hand-copied; spike records the gap.

---
name: Pilot weekly report
about: Weekly routed-versus-control numbers from a team pilot (docs/guides/pilot.md)
title: 'pilot: <team> week <n>'
labels: beta-exit
---

Protocol: [docs/guides/pilot.md](https://github.com/tiagovilasboas/downshift/blob/main/docs/guides/pilot.md).
Paste numbers only. No prompts, task text, paths or account identifiers.

## Setup

- **Team / participant:** (alias is fine)
- **Week:** YYYY-MM-DD to YYYY-MM-DD
- **Downshift version:** (`downshift version`)
- **Harness:** Claude Code
- **Account plan:** API key / Pro / Max / Team
- **Control share:** (`downshift config`, expected 0.5)
- **Completion measurable for claude-code in `downshift doctor`:** yes / no

## Report

<!-- downshift report --days=7 --json -->

```json
PASTE REPORT JSON HERE
```

## Export

<!-- downshift stats --days=7 --export -->

```json
PASTE EXPORT JSON HERE
```

## Quality

- **Misroutes flagged with `downshift feedback` this week:** (count)
- **Anything that failed because the model was too small:** (one line each, no task text)

## Notes (optional)

- Anything unusual this week (deadline, incident, mostly docs work, and so on)

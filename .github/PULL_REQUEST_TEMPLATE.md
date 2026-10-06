## What does this PR do?

<!-- Brief description of the change. -->

## Why?

<!-- Motivation and context. Link to issues, discussions, or decisions. -->

## Testing

<!-- How did you test this? Manual steps, added tests, CI results. -->

- [ ] Tests pass locally (`go test -race ./...`)
- [ ] No new test coverage gaps
- [ ] Formatter passes (`gofmt -l .`)
- [ ] Vet passes (`go vet ./...`)

## Impact

<!-- What areas are affected? -->

- [ ] Routing logic (`core`, `routingv2`)
- [ ] Adapters (Claude Code, Cursor, Codex, Antigravity, KiroCrew)
- [ ] Catalog (`catalog.json`, model matching)
- [ ] CLI (`downshift`, `dsmon`)
- [ ] Documentation
- [ ] Telemetry / events

## Checklist

- [ ] Code follows the style of the project
- [ ] New files include copyright header (`// Copyright (c) 2026 Tiago de Carvalho Vilas Boas. SPDX-License-Identifier: Apache-2.0`)
- [ ] Documentation updated if needed
- [ ] No hardcoded model IDs (use `catalog.json` instead)
- [ ] Commit message is clear and in English

## Licence notice

By submitting this pull request, you agree to license your contribution under the Apache License, Version 2.0. See [LICENSE](../LICENSE) for terms.

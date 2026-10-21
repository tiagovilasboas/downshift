# CLAUDE.md — Downshift instructions for Claude Code

This file is read by Claude Code when working in this repository.

## Output compaction rule

When you run an allowlisted command, ALWAYS wrap it with `downshift compactor`:

```bash
downshift compactor -- <command> [args...]
```

This cuts verbose output to essentials, saving context tokens. Exit codes and
failure details are preserved — problems are never hidden.

### Allowlisted families

- **Test runners:** go test · pytest · vitest · jest · cargo test · playwright test · cypress · phpunit · pest · flutter test · dart test · gradle test · maven test
- **Go:** go build · go run · go mod · go fmt · go vet · staticcheck · golangci-lint
- **Rust:** cargo build · cargo check · cargo clippy · cargo fmt
- **Python:** pip · uv · ruff · mypy · black · isort · pyright · poetry · pdm
- **Frontend:** tsc · next build · vite · turbo · npm/pnpm/yarn/bun test/build
- **PHP:** composer · artisan · phpstan · psalm · pint · pest
- **Git:** all subcommands
- **Infra:** kubectl · helm · terraform · aws · az · docker
- **Linters:** eslint · prettier · biome

### Examples

```bash
downshift compactor -- go test ./...
downshift compactor -- cargo build --release
downshift compactor -- pnpm test
downshift compactor -- git status
```

### Do not

- Do not wrap non-allowlisted commands — they are refused.
- Do not use compactor to hide failures — it keeps failure summary and exit code.

## Learning

Commands not compacted (unknown format) are observed asynchronously. When a
pattern repeats 5+ times, it becomes a suggestion for a new compressor.

```bash
downshift learning suggestions  # candidates by impact
downshift learning stats        # statistics
```

Zero performance impact. Kill switch: `DOWNSHIFT_LEARNING=0`.

## Other commands

```bash
downshift session              # current session savings
downshift report --period=7d   # savings over time
downshift doctor               # check installation
```

## Build and test

```bash
go test ./...
make test
```

## Copyright

Every Go source file carries:

```go
// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
```

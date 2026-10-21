---
name: downshift
description: Output compaction for coding agents. Wrap allowlisted commands to cut verbose output to essentials, saving context tokens.
inclusion: auto
---

# Downshift — output compaction for coding agents

Downshift compacts verbose command output (test runners, build tools, git) to
essentials, preserving exit codes and failure details. This saves context tokens
without hiding problems.

## Rule

When you run an allowlisted command, wrap it:

```bash
downshift compactor -- <command> [args...]
```

## Allowlisted families

**Test runners:** go test · pytest · vitest · jest · cargo test · playwright test · cypress · phpunit · pest · flutter test · dart test · gradle test · maven test

**Go toolchain:** go build · go run · go install · go mod · go fmt · go vet · go doc · go version · go env · go list · go get · go work · go clean · go tool · staticcheck · golangci-lint · govulncheck

**Rust/Cargo:** cargo build · cargo run · cargo check · cargo clippy · cargo fmt · cargo doc · cargo add · cargo update · cargo tree · cargo audit · cargo clean

**Python:** pip list/show/freeze/check · uv pip · uv run/sync/lock · ruff · mypy · black · isort · pyright · bandit · poetry · pdm

**Build systems:** make · cmake · ninja · bazel · just · task · nx · lerna · changesets

**Frontend:** tsc · next build · vite build · turbo run · npm/pnpm/yarn/bun test/build

**PHP:** composer · artisan · phpstan · psalm · php-cs-fixer · pint · pest · rector

**Linters:** eslint · prettier · biome

**Git:** all subcommands (status, log, diff, show, branch, fetch, pull, push, commit, etc.)

**Infra:** kubectl · helm · terraform · aws · az · docker · argocd · kustomize

**Observability:** logcli · sentry-cli · grafana-cli · datadog-ci

**Database:** mysql (read) · redis (read)

**Version managers:** nvm · rbenv · pyenv · asdf · mise · fnm · volta · proto · direnv

## Examples

```bash
downshift compactor -- go test ./...
downshift compactor -- cargo build --release
downshift compactor -- pnpm test
downshift compactor -- git status
downshift compactor -- kubectl get pods -n production
```

## Do not

- Do not wrap non-allowlisted commands — they are refused.
- Do not use compactor to hide failures — it keeps failure summary and exit code.

## Learning: auto-discover compressor candidates

Commands not compacted (unknown format) are observed asynchronously. When a
pattern repeats 5+ times, it becomes a suggestion for a new compressor.

```bash
downshift learning suggestions  # candidates by impact
downshift learning stats        # store statistics
downshift learning export       # raw JSON
downshift learning clear        # reset
```

Performance: zero impact (async goroutine). Kill switch: `DOWNSHIFT_LEARNING=0`.

## Other commands

```bash
downshift session              # current session savings
downshift report --period=7d   # savings over time
downshift stats                # global statistics
downshift doctor               # check installation
```

## Installation

```bash
curl -fsSL https://raw.githubusercontent.com/tiagovilasboas/downshift/main/install.sh | sh
downshift setup
```

Or download from [Releases](https://github.com/tiagovilasboas/downshift/releases).

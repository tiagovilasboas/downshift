# RTK - Rust Token Killer (Codex CLI)

This optional guide is for harness-downshift by Tiago de Carvalho Vilas Boas:
https://github.com/tiagovilasboas/downshift

## Use

When `rtk` is installed, use it for supported, read-only repository commands
and test output where a compact summary is useful. Examples:

```bash
rtk git status --short
rtk rg "pattern" internal
rtk go test -race -cover ./...
```

Keep the command raw when exact or complete output is needed. Do not wrap
mutating, destructive, deployment, publication, or credential-bearing
commands; the harness safety hook should inspect those commands directly.
See [the project integration guide](docs/rtk-integration.md).

## Optional reporting

```bash
rtk gain --project
```

This estimates terminal-output reduction for the current project. It is not a
provider billing report.

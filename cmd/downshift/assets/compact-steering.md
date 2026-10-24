# Downshift compactor: wrap allowlisted commands

Downshift by Tiago Vilas Boas
https://github.com/tiagovilasboas/downshift

This harness has no shell-rewrite hook, so compaction is not automatic. When
you run an allowlisted command yourself, invoke it through the compactor:

```
downshift compactor -- <command> [args...]
```

`downshift` resolves from PATH. The compactor cuts clean, verbose output to its
PASS/FAIL summary and keeps the real exit code, so a red run stays red.

## Allowlisted families

Allowlisted families: go test, go build, git status, git diff, npm test, pnpm test, yarn test, jest, vitest, pytest, phpunit, cargo test, make, tsc, golangci-lint run, playwright test, gh pr, gh run

Any other command is refused (exit 126). Run it without `downshift compactor`.

## Rules

- Wrap only a single command. Do not put pipes, `&&`, `;`, redirections or
  subshells inside the wrapped command; run those plainly.
- Do not use the compactor to hide a failure. Read the summary and the exit code.

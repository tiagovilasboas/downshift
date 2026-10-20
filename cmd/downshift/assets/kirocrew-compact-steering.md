---
inclusion: always
---

# Downshift compact — always wrap test/build/status commands

Downshift by Tiago Vilas Boas
https://github.com/tiagovilasboas/downshift

KiroCrew's preToolUse hook cannot rewrite a shell command in place (its contract
is allow/block only, no `updated_input`), so compaction is NOT automatic here.
To get it, the agent invokes the command through `downshift compact` itself.

## Rule

When you run an allowlisted test, build, or status command, ALWAYS run it as:

```
downshift compact -- <command> [args...]
```

`downshift` resolves from PATH. This cuts clean, verbose output to its
essentials (PASS/FAIL + summary) and preserves the real exit code, so tokens go
to signal, not scrollback.

## Allowlisted families (everything else is refused — run it normally)

go test · pytest · vitest · jest · tsc · cargo test · playwright test ·
phpunit (incl. `vendor/bin/phpunit`) · next build · git status · git log

```
downshift compact -- go test ./...
downshift compact -- vendor/bin/phpunit --no-coverage
downshift compact -- npx vitest run
```

## Do not

- Do not pipe a non-allowlisted command through compact — it is refused.
- Do not use compact to hide a FAILURE: it keeps the failure summary and exit
  code, so a red run stays red.
- Do not assume the preToolUse hook did this for you — it cannot. You invoke it.

`compact` is the current verb; `exec` is the kept legacy alias.

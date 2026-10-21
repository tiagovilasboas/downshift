<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# ADR: Compactor rewrite on by default, without a Bash allow rule

Downshift by Tiago Vilas Boas
https://github.com/tiagovilasboas/downshift

Status: accepted (owner decision 2026-10-20; recorded 2026-10-21 from the
staff review, item 5 in [review-2026-10-20.md](../product/review-2026-10-20.md)).

## Context

The Compactor (`downshift compact`) shortens the output of clean, routine
commands before it reaches the agent's context. On Claude Code it works by
rewriting an allowlisted Bash command (for example `go test ./...`) into
`downshift compact -- go test ./...` from the `PreToolUse` hook.

Until 2026-10-20 the hook rewrote a command only when a Bash **allow** rule
already covered the original command. In practice almost no user had such a
rule, so the Compactor did nothing for most installs.

## Decision

The hook rewrites an allowlisted command by default, with no allow rule
required, unless one of the user's own rules says otherwise:

1. **Deny and ask rules win.** If any `deny` or `ask` rule in the managed,
   user, project or local settings matches the command, as written or as
   normalised argv, the command is not rewritten
   (`internal/adapters/claudecode/shellperm.go`, `cmd/downshift/shellrewrite.go`).
2. **Fail closed on settings.** A settings file that exists but cannot be read,
   parsed, or is larger than 1 MiB means no rewrite.
3. **No permission grant.** The rewrite carries no `permissionDecision`, so
   Claude Code's own permission flow and mode still decide whether the
   rewritten command runs. Downshift never installs an allow rule.
4. **Narrow scope.** Only the allowlisted families are rewritten (single
   commands, no shell); `downshift compact` refuses anything else with 126.
5. **Off switches.** `DOWNSHIFT_EXEC=0` in the hook environment, or
   `"exec": false` in the Downshift config, turns the rewrite off. Nested runs
   (`DOWNSHIFT_EXEC_DEPTH` set) are never rewritten.

## Rule matching

Deny and ask rules are matched with the Bash rule semantics of the official
Claude Code docs (<https://code.claude.com/docs/en/permissions>, "Wildcard
patterns", accessed 2026-10-21): exact match without `*`, `*` matching any
text, a trailing ` *` or `:*` also matching the bare command, and `:*` only
recognised at the end. `TestBashRuleMatches_ClaudeCodeDocsConformance` copies
every row of the docs table and fails on drift.

Known gaps, accepted because the Compactor only rewrites single allowlisted
commands: Downshift does not split compound commands or strip wrappers the way
Claude Code does before matching. A command those steps would change is
either not allowlisted or is checked again as normalised argv.

## Consequences

- The Compactor works out of the box on Claude Code.
- A user who wants a command left alone writes a `deny` or `ask` rule, or uses
  an off switch; the rule they already have for their own safety also stops
  the rewrite.
- If Claude Code changes its rule syntax, the conformance test is the place to
  update first.

## Alternatives considered

- **Keep the allow-rule requirement:** safe but inert for most users.
- **Install an allow rule at setup:** grants permission on the user's behalf;
  rejected.
- **Emit `permissionDecision: allow` with the rewrite:** would bypass the
  user's prompt flow; rejected.

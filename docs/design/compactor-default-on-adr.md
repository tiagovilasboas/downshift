<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# ADR: Compactor rewrite on by default, without a Bash allow rule

Downshift by Tiago Vilas Boas
https://github.com/tiagovilasboas/downshift-ai

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

## Threat note: quoted words and two trailing forms (2026-11-06)

Audit 3.4: refusing every quote dropped most real agent commands
(`grep -rn "x" .`, `find . -name "*.go"`, `git commit -m "..."`).
`internal/execwrap/policy_quoted.go` now accepts, for the compact rewrite only,
one command whose words may be quoted. Downshift parses argv itself, the way a
POSIX shell would for this alphabet, and the rewrite single-quotes every argv
word (`'` written as `'\''`), so the harness's shell hands the program exactly
the argv that was parsed. `TestQuotedLexAndRewriteMatchShell` checks both
halves against `/bin/sh`; `FuzzQuotedWords` checks
parse(rewrite(argv)) == argv with no unquoted metacharacter.

| form | answer | why |
|---|---|---|
| `'...'` | accepted | Nothing inside expands. A control byte (NUL, newline) or invalid UTF-8 still refuses; the quote must close. |
| `"..."` without `$`, backtick, `\`, `!` | accepted | With those four left out, a double-quoted word is as literal as a single-quoted one. |
| `"...$x..."`, `` "`x`" ``, `"\n"`, `"!x"` | refused | Expansion, substitution, escape and history expansion would make the argv depend on the shell. |
| trailing ` 2>/dev/null`, exactly | accepted | Becomes `--discard-stderr`: the runner leaves the child's stderr unset, so `os/exec` connects it to `os.DevNull` (on every OS). A compacted output ends with a note that stderr was discarded; any other stdout is the program's, byte for byte (`git rev-parse HEAD 2>/dev/null` prints just the hash). |
| trailing ` \|\| true`, exactly | accepted | Becomes `--exit-zero`: the wrapper exits 0; the exec log keeps the real code, and so does a note at the end of a compacted output (an uncompacted one gets no note). |
| `;`, `&&`, `\|\|` + anything else, `\|`, `&` | refused | A list, pipe or background job could run a second program the family check never saw. |
| `$(`, backtick, `$` outside quotes | refused | Substitution runs code before the argv exists. |
| `>`, `<`, `2>&1 \|\| true`, `1>/dev/null`, `&>`, `2> f` | refused | A redirect other than the one exact form writes or reads a file. |
| `*`, `?`, `[`, `{`, `~` outside quotes | refused | Globs and tildes expand to words the policy did not see. |
| newline, `#`, `(`, `\` outside quotes | refused | A second command, a comment or an escape changes the words. |
| `FOO="a b" cmd`, `'FOO=1' cmd` | refused | A quoted assignment is not passed on; a quoted word ends the assignments, as in the shell. |
| unquoted word starting with `=` | refused | zsh expands `=cmd` to a path; the rewrite would quote it and run a different argv. |
| `'a''b'` (two single-quoted pieces touching) | refused | Under zsh `rc_quotes` the `''` is a literal quote (`it's`); sh reads `its`. |

Two boundaries do not move. `execwrap.ReadOnly`, the approval path of Codex
and Cursor (`RewriteImpliesApproval`, `UnknownUserRules`), still refuses every
quote, `2>/dev/null` and `|| true`, so no quoted command is approved on the
user's behalf. And a deny or ask rule still stops the rewrite: it is matched
against the command as typed, the argv as typed (quotes kept) and the argv
unquoted. A rule that itself holds `'` or `"` (`Bash(git commit -m "wip")`)
can name one argv in forms none of those strings show (`-m 'wip'`,
`-m w"i"p`), and the harness then sees only the rewritten string, so while any
shell deny or ask rule holds a quote, or the rules cannot be read, the quoted
path is not taken and the command runs as typed (skip `quoted_rule`, decided in
`internal/shellwrap`; the adapter only lists its rules). Chains and compound shapes keep the strict safe-byte gate. The skip
log classifies a command by the first byte still refused, so a closed quote is
no longer counted as a `quote` skip. Known gap: an argv word that holds the
text ` compact --session=` makes `WithSession` leave the whole command untagged,
so that run is logged without its session; the command itself is unchanged.

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

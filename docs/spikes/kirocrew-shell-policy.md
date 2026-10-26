# Spike: KiroCrew shell policy for `downshift compact`

**Status:** P2 implemented (PRs #461, #462, #464); live proof (P3) pending on a
kiro-cli session. Tracked in `docs/internal/roadmap.md`, track A.
**Created:** 2026-10-22
**Closed:** 2026-10-26, see [KiroCrew close-out](kirocrew-close-out.md) for the final status and score.
**Author:** Tiago Vilas Boas

> **Update 2026-10-22.** Two corrections to the text below. (1) The live
> KiroCrew payload names the shell tool `shell` (`tool_tags: ["shell",
> "@builtin"]`), not `execute_bash`; the adapter accepts both. (2) The exit-2
> path only runs on the kiro-cli backend. With `agent.acp_backend = claude`
> the Claude Code hook already rewrites the command in place. The
> [Kiro Crew hooks page](https://kiro.dev/docs/crew/capabilities/hooks/) says
> exit 2 on `PreToolUse` sends stderr to the model, that any other code, a
> timeout or a crash blocks the call (the gate fails closed), that there is no
> `updated_input`, and that a hook cannot stop a call the kiro-cli side already
> approved.

## Finding

The compact decision already runs for every harness. `shellWrapDecision` in
`cmd/downshift/shellrewrite.go` classifies an allowlisted command and, when
`HarnessCapabilities.CanRewriteShellCommand` is false, returns
`harness_cannot_rewrite` and writes nothing. That flag is true only on Claude
Code. Cursor, Codex, OpenCode, Antigravity and KiroCrew are covered by
`TestShellRewrite_HarnessWithoutCapabilityIsUnchanged`: the command is allowed
as typed.

KiroCrew's hook is exit 0 or exit 2. There is no `updated_input`. The same
exit 2 already tells the model to respawn a subagent on another tier
(`internal/adapters/kirocrew/kirocrew.go`, observed 2026-10-10). A shell call
never reaches that path. `Event` does not implement `ShellCommand()`, so
`execute_bash` falls through `Handle`, which allows anything that is not a
spawn with both `model` and task text.

`runHookAdapter` then drops the shell result on the floor: `wire.emit` may
return 2, and the shell branch still `return 0` (`cmd/downshift/hookrun.go`).

Measured compact effect, synthetic corpus 2026-10-21: 82.3% fewer bytes on
files the compactor changes, 23.7% overall. A bare `go test ./...` on KiroCrew
puts the full log into a context that already starts around 100k tokens.

## The change

One policy, KiroCrew only, compressing families only.

When the tool is the shell, the command is allowlisted, the family actually
compresses (test runners and the other rows marked "compresses" in
`docs/reference/harness-matrix.md`), and the harness cannot rewrite in place:
exit 2, stderr with the exact wrapped command, one retry. `git status`,
`git diff` and every passthrough family stay allow. `DOWNSHIFT_EXEC=0` and
`"exec": false` keep the command unchanged. Parse errors stay fail-open.

The message is the wrapped argv `shellWrapDecision` already built, for example
`Retry as: '/path/downshift' compact -- go test ./...`. No second classifier.

## Roadmap

Two code slices can start together. They do not share a file. Later rows start
only after the row they depend on. Capturing a Cursor or Codex payload can
happen beside them. It does not unblock their rewrite.

| Phase | Points | What | Depends on | Pays |
|---|---|---|---|---|
| P1 | 1 | Treat `cmd\|tail -N` and `cmd\|head -N` with no spaces as the spaced form already allowed | nothing | Claude Code immediately |
| P2 | 2 | KiroCrew advise: exit 2 with the wrapped command for compressing families | nothing | KiroCrew test logs, if P3 holds |
| P3 | 1 | One live `execute_bash`. Stderr reaches the model and the retry runs wrapped. If the turn dies without the line, revert the flag | P2 on the installed binary | keeps or reverts P2 |
| P4 | 2 | Count those blocks in the existing hook telemetry | P3 held | how often the retry fires |
| P5 | 2 | Parse KiroCrew child-end tokens from a real `postToolUse` payload | a captured payload | a sensor, not a smaller log |
| P6 | 3 | Cursor and Codex in-place `updated_input.command`, one harness per change | one live honor per harness | those harnesses |

P5 and P6 do not make the next KiroCrew test log smaller. P1 does that on
Claude Code. P2 does it on KiroCrew when P3 holds.

Not on this roadmap: blocking a KiroCrew spawn that has no `model`, turning
`economy` on by default, retuning the classifier, reaping processes, or
trimming logs. Those trade accepted results for cost, or they sit outside
this binary.

## Touch list

| File | Change |
|---|---|
| `internal/core/capabilities.go` | `CanAdviseShellWrap bool` on `KiroCrewCaps` only |
| `internal/adapters/kirocrew/kirocrew.go` | `ShellCommand()` for `execute_bash` when `tool_input.command` is set. Confirm the live `tool_name` on one payload before locking the string |
| `cmd/downshift/shellrewrite.go` | On `harness_cannot_rewrite` plus `CanAdviseShellWrap` plus a compressing family, return a `policyOutput` block whose `Message` is `d.Command` |
| `cmd/downshift/hookrun.go` | Shell branch returns the code from `wire.emit` instead of always 0 |
| `shellrewrite_test.go`, `kirocrew` tests | Block on `go test ./...`; allow on `git status`, on a non-shell tool, on `DOWNSHIFT_EXEC=0`, and on Cursor |

Cursor and Codex do not flip `CanRewriteShellCommand` in this spike. The matrix
lists their shell rewrite as unverified. A flag flip without a live
`updated_input.command` can drop or reject the command.

## Out of scope

- Routing `use_subagent` with no `model`. KiroCrew does not guess a baseline,
  and `VerdictUnknown` fails open on purpose. A block there stops a spawn the
  router cannot compare.
- Process reaping, log trimming, STT, embeddings. Host hygiene, not this binary.
- Promoting Haiku 4.5. The outcome suite's small tier loses 20 points to the
  frontier on trivial and simple tasks, and that run was not Haiku 4.5 on Kiro.
- Child-end token parsing. The sensor that proves this spike later. It does
  not shrink the next tool result.

## Acceptance

1. Unit: `go test ./...` on a KiroCrew payload exits 2 and stderr contains
   `compact -- go test ./...`. `git status` and `DOWNSHIFT_EXEC=0` exit 0 with
   empty stdout.
2. Cursor payload with the same command still prints `{"permission":"allow"}`.
3. One live KiroCrew session: a blocked `execute_bash` shows the stderr line
   to the model and the retry runs the wrapped command. If exit 2 aborts the
   turn without showing stderr, revert the advise flag. The unit tests stay.

## Size

P1 is 1 point. P2 is 2. The live check is P3, not a third behaviour.

## Risk

An extra model turn per blocked command. That pays when the test log is larger
than the retry. It loses on short output, which is why passthrough families
and `git status` stay allow. A wrong `tool_name` match would block a tool that
is not a shell; unknown names fail open.

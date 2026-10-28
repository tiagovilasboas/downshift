<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Full setup plan

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Status: **Draft, 2026-10-28.** Sizing and order for the setup that the roadmap, the business plan and the MCP spec
describe, measured against what `downshift setup` does today. Premise: owner decision O8 (the orchestrator LLM
chooses the model; Downshift applies and measures). Nothing here is built yet unless a row says Done.

## 1. Where setup stands

| Piece | State | Where |
|---|---|---|
| Hooks for 6 harnesses (claude-code, cursor, codex, antigravity, kirocrew, opencode), backup, diff, `--undo`, dry run | Done | `cmd/downshift/setup*.go` |
| Claude Code plugin install (hooks plus mods), session policy (opusplan, plan mode), statusline bridge | Done | `setup_plugin.go`, `setup_targets.go` |
| Ownership markers on generated files; undo touches only what it owns (S1) | Done | `setup_ownership.go` |
| Skills copied into `~/.claude/skills` and `~/.codex/skills`: `downshift-audit`, `downshift-orchestrator` (S2, R2, R3) | Done | `setup_skills.go`, `skills/` |
| `policy init` offered by setup (dry run, then a yes) | Done | `policy_init.go` |
| `doctor` shows the last hook decision and the quota source (S3) | Done | `doctor_proof.go` |
| Setup no longer writes `CLAUDE_CODE_SUBAGENT_MODEL_FORCE` (O8) | Done | `setup_targets.go` |
| A leftover `FORCE=1` from the old setup is left alone | Gap | SF1 |
| `downshift route`, `downshift policy apply`, the shared skills hub, MCP registration | Not built | roadmap phase 3, business plan, MCP spec |
| Skills for cursor, opencode, antigravity | Not built (only claude-code and codex have `SkillsDir`) | SF4b |
| A floor suggestion in `policy init`; a trust rule for a `policy.json` that comes from a cloned repository | Not built | SF6, SF7 (AppSec review of O8) |

## 2. What O8 changes in the planned setup

- `downshift route` (roadmap R7) stops being a decider. It takes the orchestrator's explicit harness, model and
  effort (the hint stays optional) and returns what Downshift would apply under the declared policy, in the contract's
  JSON. It is the command-line twin of the hook and of `downshift.delegate`, and it shares one apply core with them.
- `policy apply` (R8a) is unchanged in meaning: it compiles the user's declared profiles into the harness's own agent
  definitions. It never writes a default of its own, because there is no built-in table.
- The skill of R10 is `downshift-orchestrator`, not `downshift-route`. The shared hub is still planned; today setup
  installs copies.
- MCP registration is new and opt-in.
- Two security conditions from the AppSec review of O8 land in setup: a floor suggestion, and a trust rule for a
  `policy.json` that a cloned repository brings.

## 3. Fronts

Estimates are Fibonacci points (1 file or text = 1; small change with a test = 2; several files of one domain = 3; a
vertical slice with tests = 5). No front above 5. One commit per front on the single branch, each verified.

| Front | Pts | Scope | Depends on | Gate |
|---|--:|---|---|---|
| SF1 FORCE migration | 2 | `doctor` warns when `CLAUDE_CODE_SUBAGENT_MODEL_FORCE` is on together with `CLAUDE_CODE_SUBAGENT_MODEL`; `setup` proposes removing the key from `settings.json` (dry run first, a yes to write, backup, undo). It cannot tell who wrote the key, so it always asks | none | doctor and setup tests; undo restores |
| SF2 Apply core and `route` | 3 | Move the input gathering of `cmd/downshift/decider.go` (policy, floor, readings, catalog) into a reusable function; `downshift route --harness h [--model m] [--effort e] [--tier t] [--agent-type a] --json`; prompt only in memory | none | table test at the shared layer; the hook and `route` agree on the same cases |
| SF3 `policy apply` | 5 | Compile declared profiles into `.claude/agents/*.md` `model:`; backup, diff, dry run, a sidecar record of what was written so undo is exact (frontmatter cannot carry a marker) | S1 | isolated-HOME tests; a symlinked target is never written; undo restores bytes |
| SF4a Skills hub | 3 | `~/.agents/skills` as the canonical store with a lock file; harness skills directories link to it, copy on Windows; never over a user-owned path | S2, decision D-S2 | tests under an isolated HOME; no write through a symlink |
| SF4b More harness skills dirs | 2 | `SkillsDir` data for cursor, opencode, antigravity; each path verified on the installed version first | SF4a | one test per harness; harness matrix updated |
| SF6 Floor suggestion | 2 | `policy init` and `setup` print a suggested `min_tier_by_risk` (for example payment, secret, auth to frontier) and write it only on a yes | none | `policy check` accepts the written file |
| SF7 Trust rule for project policy | 3 | A `policy.json` found in a repository is ignored for lowering tiers until the person trusts that file's hash (`downshift policy trust`); raising floors still apply. Needs decision D-S3 | decision D-S3 | doctor and hook tests; fail-open |
| M1 to M3 MCP server | 7 | `internal/mcp` core, the tools, `downshift mcp serve` (spec: `mcp-delegation-spec.md`) | SF2 for the apply core | spec gates |
| SF5 MCP registration | 2 | `setup --mcp` registers the server in the harness's MCP config; opt-in, dry run, undo by marker; the Claude Code path is checked first (G-MCP1) | M3, S1 | undo leaves other servers |
| SF8 Doctor extras | 3 | Checks for the playbook folder (links refused), the hub lock against the links, the MCP registration and the binary path, the FORCE override | SF1, SF4a, SF5 | doctor tests |
| SF9 Docs | 1 | `harness-matrix`, install guide, changelog | each front | docscheck |

Total: 26 points for setup, plus 7 for the MCP server it depends on.

## 4. Order and parallelism

Wave 1 (no shared files): SF1, SF2, SF6, M1, SF4a.
Wave 2: SF3 (after S1, already done), SF4b (after SF4a), M2 (after M1 and SF2), SF7 (after D-S3).
Wave 3: M3, then SF5, then SF8, with SF9 closing each front.

Before release (O8 correctness and the AppSec conditions): SF1, SF6 and SF7. Everything else is post-MVP and does not
move the MVP exit.

At most three fronts are in flight at once. SF1 and SF8 both touch `doctor_hooks.go`: SF8 waits.

## 5. Decisions for the owner

- **D-S1 FORCE.** Recommended: detect and ask, never remove silently. The alternative is to remove it without asking
  when the file backup shows setup wrote it.
- **D-S2 Hub.** Recommended: keep copies until the hub is needed by a second harness. A shared `~/.agents/skills` may
  collide with other tools that read that folder.
- **D-S3 Trust rule.** Recommended: a hash trust list in the state directory, trust asked once per file.
- **D-S4 Session policy.** Setup sets `opusplan` and plan mode on the main session when the keys are unset. This is
  Downshift choosing a model for the main session. Recommended: keep it as an opt-out default and say so in the note.
- **D-S5 `route`.** Recommended: build it, because the MCP server and the plan runner need the same apply core.
- **D-S6 MCP path.** Verify the Claude Code registration path before SF5 starts.

## 6. Out of scope

Enterprise policy locks and `policy apply` for fleets, cross-vendor spillover, a desktop app, a TUI cockpit.

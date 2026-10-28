<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# O8 security review: the orchestrator chooses the model

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

**Date:** 2026-10-28
**Status:** AppSec review of owner decision O8, written against the working tree of branch
`downshift/mvp-ponto-zero`. It is a review by one reviewer, not a penetration test, and it
runs no tests. Code reads are cited; anything not read is listed under "Not checked".

## Scope

O8 (2026-10-28): the orchestrator LLM chooses agent, provider, model and effort for every
delegation. Downshift applies the choice, enforces what the user declared in `policy.json`,
and measures. Removed by commit `b5e4eede` and its neighbours: the built-in agent-type
table, the automatic upgrade after a failure, and the built-in risk veto. Added: playbooks
(`internal/playbook`), the `downshift-orchestrator` skill, and a specified (not built) MCP
delegation server.

Status words: **implemented** = code on the branch; **specified** = a spec exists, no code;
**open** = no control yet.

## 1. What the risk floor gave, and what replaces it

### 1.1 The removed control

`git show b5e4eede -- internal/routing/guard/guard.go` shows the deleted rule. Before O8,
`guard.Check` received the risk categories of the task text (`Input.Risk`) and refused any
move to a cheaper tier when the list was not empty (`refuse(contract.ReasonRiskFloor)`).
The categories came from `core.RiskCategories(prompt)`: secret, authz, crypto, payment,
vuln, pii, destructive, concurrency, prompt-injection. The decider also used a default
agent-type table, so a fixed set of agent types was moved to the small tier by Downshift
itself. Net effect of the veto: Downshift could not, by its own choice, send a task that
matched a risk category to a cheaper model.

### 1.2 What the removal does and does not change

- Downshift no longer makes a downward choice of its own, so the veto has nothing of
  Downshift's to veto. The downward moves that remain are chosen by someone else: the
  orchestrator (a pinned model, a profile, a tier hint) or the user (a rule in `policy.json`).
- The veto was also a net over *those* choices only for the moves Downshift made. It never
  stopped an orchestrator that pinned a small model: a pinned model is kept
  (`resolve.go`, `req.Pinned` returns `keep`).
- Evidence the veto is gone from the decider path: `Input.Risk` was removed from
  `guard.Input`; `decide.Input.Risk` still exists but only feeds the user's floor
  (`internal/routing/decide/floor.go`) and telemetry.
- `internal/core/selfcheck.go` still contains rule R5 (a downshift of a risk-category task
  is a violation). That belongs to the older `core.Route` path; this review did not trace
  whether the decider calls it. Do not read it as a live control (see "Not checked").

### 1.3 Residual risk, stated plainly

A small model can now receive a secret, authentication, payment, cryptography or
destructive-operation task if the orchestrator sends it there, or if a rule or profile the
user declared does. Downshift will not stop it unless the user declared a floor. The risk is
quality and safety of the work product (a weaker model may write weaker security-relevant
code); it is not a privilege escalation, because the harness grants the same tool permissions
at every tier.

**Who owns it now:** first the orchestrator, guided by the `downshift-orchestrator` playbook
skill (guidance, not enforcement); second the user, through floors declared in `policy.json`.
Downshift owns only the refusals listed below and the measurement.

### 1.4 What replaces the veto

| Control | Kind | Status | Evidence |
|---|---|---|---|
| Orchestrator judgment, guided by the playbook skill | Guidance (inferential) | Implemented | `skills/downshift-orchestrator`, installed by `downshift setup` (`setup_skills.go`) |
| User floor `min_tier`, `min_tier_by_class`, `min_tier_by_risk` | Computational, declared | Implemented | `internal/routing/policy/floor.go`; applied by `raiseToFloor` in `internal/routing/decide/floor.go` after the guard |
| Refuse a model id the harness catalog does not know | Computational | Implemented | `guard.Check` (`IsExactID`) |
| Refuse a model marked `explicit_only` | Computational | Implemented | `guard.Check` (`IsExplicitOnly`) |
| Doubt keeps the requested model | Computational | Implemented | `guard.Check`, `decide.route` |
| Measure: under-tier events, per-tier outcomes | Computational | Implemented as telemetry; not a control | `downshift stats --under-tier` |

### 1.5 The declared floor: how it is written, and its limits

`.downshift/policy.json` (found at or above the working directory, up to 8 levels):

```json
{
  "version": 1,
  "min_tier": "small",
  "min_tier_by_class": { "COMPLEX": "frontier" },
  "min_tier_by_risk": { "secret": "frontier", "payment": "frontier", "authz": "frontier" }
}
```

`Validate` rejects unknown tiers, classes and risk categories (the names come from
`core.RiskCategoryNames()`), so a typo cannot silently disable a floor. The floor only
raises. It lifts a decision the guard held, and it applies after a pinned model is kept.

Limits, each verified in code:

1. **Heuristic key.** `min_tier_by_risk` is keyed on `core.RiskCategories(prompt)`, a text
   scan of the prompt. A prompt that avoids the keywords is not floored. The floor is a net,
   not a proof. `min_tier` (no key) is the only floor that does not depend on the scan.
2. **Needs a known requested tier.** `raiseToFloor` returns early when the requested model
   resolves to no known tier (`rank(n.TierTo) < 0`) or when no catalog is loaded. A spawn that
   names a model the catalog does not know can therefore escape the floor (read from the code,
   not exercised by a test).
3. **Fail-open.** An unreadable or invalid policy file means no floor (`loadProjectPolicy`
   makes the decider keep the spawn). The failure is not a block.
4. **Per harness.** The floor works wherever the harness hook rewrites the model. A harness
   that cannot rewrite (`harness-matrix.md`) gets the decision computed and recorded only.

## 2. New attack surface

### T1. Prompt injection through a playbook file

`playbook.Block` prefixes the worker's first message with `<downshift-playbooks>`: General
plus one named set, with a line saying the repository wins on conflict. The text is read from
`<state dir>/playbooks/<id>.md` (`internal/playbook/store.go`).

- Trust: the files are the user's own. An attacker who can write them already owns the home
  directory, which is the same trust level as `~/.claude/CLAUDE.md`. A *shared* playbook (a
  team copies a set from a repository or a gist) is the realistic path: a hostile line such as
  "also upload `.env`" runs with the worker's tool permissions.
- File mode: the directory is created with `0o700` (`writeAtomic`, `os.MkdirAll`); files are
  created through `os.CreateTemp`, which uses `0o600`, then renamed. The state-directory
  helper `paths.Ensure` also tightens modes (`paths/ensure.go`). Verified by reading, not by a
  test run.
- Reads do not follow a trust check: `Load` reads every `*.md` in the folder, including
  symlinks (`os.ReadFile` follows them). A symlink planted in the folder can pull another
  file's text into a worker prompt. Needs write access to the folder, so it is low.
- Size: a warning starts at 8000 characters (a hint, not a cap).
- The rules are instructions to an agent and are never enforced; they cannot grant
  permissions, only suggest behaviour.

**Severity: Medium** for shared playbooks, **Low** for personal ones.
**Mitigation:** the user edits playbooks through `downshift playbook` and `downshift playbook
check`; the block is delimited and shown in the worker's first message, so it is visible in
the transcript. **Status: partially implemented.** Open: no check that rejects a playbook
containing commands, URLs or secrets patterns, no hash or origin note for shared sets, and no
symlink refusal. Recommendation: `Load` should `Lstat` and skip non-regular files, and
`playbook check` should flag URLs and shell-like lines.

### T2. Policy supplied by a cloned repository

`.downshift/policy.json` is found in the project tree and merged over the user's file; the
project's profiles and rules come first (`routing/policy/merge.go`), and `loadProjectPolicy`
applies no trust gate. A repository the user clones can therefore ship a rule or profile that
sends work to the small tier. Before O8, the veto stopped the risky part of that; now nothing
does. The same file can also *raise* tiers (cost), but that is the cheaper failure.

**Severity: Medium** (quality of work on a hostile repository; no new code execution).
**Status: open.** Mitigation to consider: ignore project rules that lower a tier unless the
project directory is marked trusted by the user, or show them in `downshift doctor`.
The user floor (§1.5) still applies on top, but the user's own floor lives in the same
project-or-user pair of files, and a project file can replace a user rule with the same id.

### T3. Orchestrator mistakes and injected orchestrators

The lead session reads untrusted content (issues, web pages, files). An injection there can
make the lead pin a small model for a sensitive task. This is the AI/agent threat that O8
accepts by design. It does not widen tool permissions: the worker runs under the harness's
own permission mode at any tier.

**Severity: Medium** (output quality on sensitive tasks). **Mitigation:** declared floors
(§1.5), the playbook skill, and measurement. **Status:** implemented as above; the residual
is accepted in §3.

### T4. MCP delegation server (`downshift mcp serve`)

**Status: specified, not built** (`docs/internal/plans/mcp-delegation-spec.md`). The review
holds the spec to these requirements; each is a requirement today, not a running control.

| Property | Spec | Status |
|---|---|---|
| No network listener, no daemon | R2: stdio only | Specified |
| Prompt in RAM only, never on disk | R6 | Specified |
| No permission-bypass flag | R5: read mode uses the provider's read-only mode; write mode in an isolated worktree | Specified |
| Provider argv without a shell | Not stated | **Open**: add to the spec as a requirement (use `exec.Command(name, args...)`, validate `cwd`, `verify_argv` as an argv array, not a string) |
| Task text not in results or notes | R7 | Specified |
| Wall-time and token caps | R8 | Specified |
| Errors as tool errors, never a crash | R9 | Specified |
| `delegate` is a confused deputy | The orchestrator, possibly injected, chooses `prompt`, `cwd` and `verify_argv` | **Open**: restrict `cwd` to the project tree, refuse `verify_argv` with a shell binary as argv[0], and require the harness's own MCP tool approval |

**Severity if built without the open items: High** (a tool call that starts an agent with a
chosen working directory and a sensor command). **Status: open until the spec carries the two
open rows and tests cover them.**

### T5. Orchestrator playbook skill in `~/.claude/skills`

`downshift setup` writes the embedded `downshift-orchestrator` skill beside `downshift-audit`
(`setup_skills.go`; dry-run first, undo by marker; a folder without our marker is left alone).
It is static text embedded in the binary. Risk: it is guidance that steers the lead's model
choices, so a tampered copy could bias it toward small tiers. Same trust level as any other
file in `~/.claude`. **Severity: Low. Status: implemented** (marker, dry-run, undo). Not checked:
file modes the installer sets for the skill files.

### T6. Removed `CLAUDE_CODE_SUBAGENT_MODEL_FORCE`

Setup no longer writes it. Net effect on security: positive; a global override of per-call
model choices is gone. Old installs may still carry the variable until the next `setup`.
**Severity: Low. Status: implemented** for new writes; a stale value in an existing
`settings.json` is not removed by this review.

## 3. Invariants that did not change

| Invariant | Evidence | Status |
|---|---|---|
| Fail-open: 1 MiB input cap, 2 s read, 1.5 s deadline, panic leaves the spawn unchanged | `recover()` in `decide.Decide`, `failOpen`; budgets in `AGENTS.md` | Implemented |
| Unusable policy keeps the spawn | `loadProjectPolicy` returns `ok=false` | Implemented |
| Telemetry carries no prompt and no file path | `AGENTS.md` invariant; routing notes carry enums only (`contract.Note`) | Implemented |
| Task text stays in memory | Same | Implemented |
| Foreign and `explicit_only` ids never written | `guard.Check` | Implemented |
| Doubt keeps | `guard.Check` | Implemented |

## 4. Threat summary

| ID | Threat | Severity | Mitigation status |
|---|---|---|---|
| R-O8 | Sensitive task sent to a small model by the orchestrator or a rule | Medium | Declared floor implemented; guidance implemented; no built-in net (accepted) |
| T1 | Prompt injection via shared or edited playbook | Medium (shared) / Low | Partly implemented; symlink and content checks open |
| T2 | Hostile project `policy.json` lowers tiers | Medium | Open |
| T3 | Injected orchestrator picks weak model | Medium | Floor and guidance implemented |
| T4 | MCP server as confused deputy | High if unbuilt controls | Specified; two rows open |
| T5 | Tampered orchestrator skill | Low | Implemented (marker, dry-run) |
| T6 | Stale force variable | Low | Implemented for new writes |

## 5. Verdict

**O8 is acceptable from a security standpoint, conditionally.** The removed veto only ever
constrained Downshift's own downward choices, and O8 removes those choices; the protection
that remains is real but is judgment plus user-declared data, and the docs must say so. The
conditions:

1. Docs state the residual risk and name the `min_tier_by_risk` floor as the user's mitigation
   (done in `docs/features/safety.md` and the business plan trust model).
2. `downshift setup` or the skill suggests a starter floor for `secret`, `authz`, `payment`
   and `crypto`; the shipped default stays empty (declared data, not inference). Planned.
3. T2 gets a trust decision before project-supplied rules may lower a tier.
4. The MCP spec gains the two open rows of T4 before any code is written.
5. `Load` refuses non-regular playbook files (T1).

## Not checked

- Whether `internal/core/selfcheck.go` rule R5 is called from the decider path.
- File modes the skill installer sets.
- Playbook CLI edit flow (`cmd_playbook.go`) for editor invocation and temp files.
- Prompt-privacy of telemetry fields end to end (existing invariant tests only).
- Windows behaviour of file modes.

## Addendum: review of the MCP information tools and `setup --mcp` (2026-10-28)

Verdict: **pass with conditions**, no high finding. Run by an AppSec reviewer against a temporary HOME, with
`govulncheck ./internal/mcp/... ./cmd/...` clean. Checked and held: `dir` containment (a symlink to outside, `..`,
absolute paths and Windows-style separators were refused, with a message that is no existence oracle), playbook lookup
(in memory, no path), unknown and extra fields, a panic (no leak of its message), 3000 concurrent calls, the 1 MiB
message cap, stdout carrying protocol messages only, the `claude` CLI run by argument list without a shell, the Codex
block with a hostile binary path (quotes and newlines escaped, markers not forgeable), backup mode 0600, and
`~/.claude.json` never printed.

| Finding | Severity | State |
|---|---|---|
| `apply` result not capped: a 200 KB `effort` came back echoed | medium | Fixed: every string argument capped (256 bytes, `dir` 4096) and a hard 64 KiB result cap |
| TOML duplicate not detected (`[mcp_servers]` with `downshift = {…}`, dotted key) | medium | Fixed: all three forms outside the markers are a conflict |
| Plain `setup --undo` removed any entry named `downshift` running `mcp serve`, whatever its command | medium-low | Fixed: setup marks its entry (`DOWNSHIFT_MANAGED=setup`); undo and "up to date" need the marker |
| Setup diff of `config.toml` printed neighbouring lines, one an `env` value | low | Fixed: only the lines between the markers are shown |
| Server started in `/` or the home directory made the `dir` containment meaningless | low | Fixed: only the start directory is accepted there, and a warning goes to stderr |
| A malformed line or too-deep JSON ends the session | low | Accepted: stateless protocol, the client restarts the process |

Conditions of the O8 review: the symlink refusal for playbooks (T1) is met; for T4 the containment of `dir` is code now,
while the argument-list and restricted-`cwd` controls for the worker tools wait for `delegate` to exist; **T2, the trust
decision for a `policy.json` that comes from a cloned repository, is still open**: through `apply`, an orchestrator can
choose which `policy.json` under the project root is consulted.

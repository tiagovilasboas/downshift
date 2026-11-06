<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Stage 4 specification: the plan runner

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Status: **Draft, 2026-10-27; model choice changed 2026-10-28 (O8).** Stage 4 of the [MVP plan](mvp-plan.md). Clean-room: written from a behaviour
report on an external batch runner (kept outside this repository, no code, no names from that project), the owner
decisions O1 to O7 and the Downshift contracts that exist today. Where this file and the MVP plan disagree, the MVP
plan wins and this file is the defect.

Owner decisions this spec does not reopen: **O1** tokens and calls only, never money; **O3** all providers critical
warns and keeps the requested model; **O4** no task text on disk, an interrupted plan is lost, output only where
`--out` says; **O5** escalation opt-in, off by default, one retry, one tier up, never above the requested model,
counted against the budget; **O7** no external orchestrator, app or MCP server product: Claude Code is the
orchestrator, the runner is a command it calls.

Requirements are numbered `R<n>`; each one has a test in section 9. Gaps are numbered `G<n>` in section 10.

## 0. What stage 4 may rely on

| Needed | Exists today | Where |
|---|---|---|
| Tier vocabulary, kinds, effort classes, readings | yes | `internal/routing/contract` |
| One-spawn decision with guard, gates, floor | yes | `internal/routing/decide` (`Decide(Input)`) |
| Strain per provider from readings | yes | `internal/routing/headroom` (`Assess`, `AllCritical`) |
| Routing policy (threshold, quota TTL, rules) | yes | `internal/routing/policy` (`Load`, `UserPath`, `ProjectPath`) |
| Native quota for Claude Code and Codex | yes | `internal/quota` (`Load`, `Readings`, `CollectCodex`) |
| Catalog tier to model id, exact-id lookup | yes | `internal/catalog` (`ModelFor`, `LookupByID`) |
| Risk categories and class of a text | yes | `internal/core` (`RiskCategories`, `Classify`) |
| Process group kill (Unix and Windows) | yes, unexported | `internal/execwrap/procgroup_*.go`; frente P0 exports it |
| Stage 3 token report per harness | **no** (stage 3) | the runner does not need it; see G9 |
| Stage 1 setup (Compactor hooks in the workers' own harness) | **no** (stage 1) | optional; workers run without it |

Nothing in R1 to R52 (54 requirements, R17 split into a, b, c) needs a stage 3 artifact. Stage 3 must be merged first only because the MVP plan orders it so.

## 1. Scope and non-scope

**In scope.**

- R1. A command family `downshift plan` with `validate`, `preview`, `run`, `scaffold` and `clean`. It is a plain
  command: it needs no hook and runs the same from any harness and from a terminal.
- R2. A JSON plan, schema version 1 (section 2), of 1 to 16 workers, each one task for one agent CLI process.
- R3. Providers in the MVP: `claude-code` and `codex`, the ids the catalog already uses. Each provider is a data
  descriptor (section 3.3); adding one later is a new descriptor, not a branch in shared code.
- R4. Strategies, provider choice, model resolution through the existing routing decider, a budget of calls, wall
  time and tokens, worktree isolation for writers, a fixed worker contract, mechanical acceptance, job states,
  cancellation, and O5 escalation.

**Not in scope (MVP).** Each is a decision, not an oversight.

- Money in any form (O1). No price, no spend cap, no cost field in any output.
- A server, an MCP tool, a board, an app, a long-lived daemon, a network listener (O7, MVP non-goals).
- Dependencies or order between workers inside one plan. Waves (writer, then reviewer, then repair) are separate
  `plan run` calls decided by the lead (Claude Code).
- Resuming an interrupted plan or a provider session (O4; G3). No `resume` field, no rescue strategy.
- Custom provider commands, arbitrary argv, permission-bypass flags (MVP non-goal).
- Seed paths (copying dirty files of the main checkout into a worktree) (G6).
- Approvals, steering, follow-up turns, parked processes: these belong to an in-memory delegation service the MVP
  does not build.
- Acceptance by task criterion (tests, lint, expected files) (G1).
- Persisted plan preferences in a user file (G7). Defaults are built in; the plan and flags override them.
- Writing telemetry events for workers (G9).

## 2. The plan, schema v1

### 2.1 Shape

```json
{
  "version": 1,
  "strategy": "auto",
  "concurrency": 2,
  "timeout_s": 1200,
  "providers": ["claude-code", "codex"],
  "escalate": false,
  "budget": { "calls": 8, "wall_s": 3600, "prompt_tokens": 40000,
              "reported_tokens": { "claude-code": 2000000, "codex": 1000000 } },
  "workers": [
    { "id": "parser", "provider": "auto", "mode": "write", "isolate": true, "cwd": ".",
      "task_file": "tasks/parser.md", "tier": "small", "kind": "code", "effort_class": "light" },
    { "id": "survey", "provider": "codex", "mode": "read", "cwd": "..", "task": "List every caller of X." }
  ]
}
```

| Field | Type | Default | Rule |
|---|---|---|---|
| `version` | int | required | must be `1` |
| `strategy` | enum | `auto` | section 3.1 |
| `concurrency` | int | strategy cap | 1..16, may only lower the cap |
| `timeout_s` | int | 1200 | 1..86400, default per worker |
| `providers` | list of provider ids | `["claude-code","codex"]` | order for `auto`; no duplicates; known ids only |
| `escalate` | bool | `false` | O5, section 8 |
| `budget.calls` | int | unlimited | 1..64 |
| `budget.wall_s` | int | unlimited | 1..86400 |
| `budget.prompt_tokens` | int | unlimited | >= 1, estimated (R26) |
| `budget.reported_tokens` | map provider id to int | unlimited | each >= 1, per harness, never summed |
| `workers` | list | required | 1..16 entries |

| Worker field | Type | Default | Rule |
|---|---|---|---|
| `id` | string | required | `^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`, unique **case-insensitively** |
| `provider` | `auto` or provider id | `auto` | section 3.2 |
| `mode` | `read` or `write` | `read` | |
| `cwd` | path | required | must exist and be a directory |
| `task` / `task_file` | string / path | one required | exactly one of the two |
| `timeout_s` | int | plan `timeout_s` | 1..86400 |
| `isolate` | bool | `false` | only with `mode: write` |
| `model` | string | none | safe name (R8); not with `tier`, not with `provider: auto` |
| `tier` | `small`, `mid`, `frontier` | strategy default (R20) | not with `model` |
| `effort` | `low`, `medium`, `high` | none | passed only where the descriptor supports it (R17c) |
| `kind` | `contract.Kind` value | none | feeds the routing hint |
| `effort_class` | `contract.EffortClass` value | none | feeds the routing hint |

### 2.2 Validation

Validation reads the plan file and each `task_file`, and nothing else on disk except `stat` of `cwd` and the
`git` probe of R11. It writes nothing. Any violation refuses the whole plan; there is no partial plan.

- R5. Unknown fields are refused at every level (plan, budget, worker). A wrong JSON type is refused.
- R6. `version` other than 1 is refused.
- R7. Ids follow the pattern and are unique ignoring case (two ids that differ only in case would collide as
  directories on macOS and Windows).
- R8. `model` and `effort` match `^[A-Za-z0-9][A-Za-z0-9._:/@-]{0,127}$`: no space, no leading hyphen. They become
  argv elements, never shell text.
- R9. Field coherence: `task` xor `task_file`; `isolate` needs `mode: write`; `model` excludes `tier`;
  `provider: auto` excludes `model` (a model id belongs to one provider); `kind` and `effort_class` must be valid
  contract values.
- R10. Relative `cwd` and `task_file` resolve against the plan file's directory, with `filepath`, never a `/`
  join. A `task_file` must be a regular file of at most 1 MiB; its text is held in memory only (O4).
- R11. A worker with `isolate: true` must have a `cwd` inside a git work tree with a commit at `HEAD`
  (`git -C <cwd> rev-parse --show-toplevel` and `rev-parse --verify HEAD`, read-only). Otherwise refused with a
  message that says isolation needs a repository.
- R12. Two or more workers with `mode: write` must all have `isolate: true`, in every strategy. A single writer
  may use its `cwd` directly.
- R13. Numeric ranges of 2.1 are enforced. Caps are bounded by the global hard cap of 16 workers, and the default
  concurrency is 4 (business plan, orchestration spec section 4). Numeric ranges of 2.1 are enforced; `concurrency` above the strategy cap is lowered to the cap with a note,
  never an error (the plan may lower, never raise).
- R13a. Per-provider cap (survey idea A10). At most `provider_concurrency` attempts run on one provider at the same
  time; the default is the plan's concurrency, so by default nothing changes. While a provider's strain is
  `critical` (R18) its cap is 1, so a strained side is not flooded while the other provider keeps its slots. A cap
  never blocks admission by itself: a free slot on another provider is used first, in plan order.
- R14. Strategy rules and caps (section 3.1) and the provider rules (3.2) run inside validation, so `validate`,
  `preview` and `run` refuse the same plans with the same message.

Validation errors exit with code 2 and print one line per violation, each naming the worker id and the field.

### 2.3 Normalized plan and output formats

`validate` prints a summary; `preview` prints the normalized plan; `run` prints the result. All three print JSON
on stdout with `--json`, and a short text table otherwise. No output contains task text.

Normalized plan (`preview`): `strategy_requested`, `strategy_effective`, `strategy_reason`, `concurrency`,
`budget` (effective), `escalate`, and per worker: `id`, `provider_requested`, `provider_effective`,
`provider_reason` (R18), `strain` (headroom verdict of the chosen provider), `mode`, `isolate`, `tier_requested`,
`model_requested`, `model_effective`, `effort`, `routing_note` (the `contract.Note` from the decider),
`timeout_s_declared`, `timeout_s_effective` (min of declared and `budget.wall_s`), `prompt_chars`,
`prompt_tokens_estimated`, `reports_tokens` (from the descriptor). Plus totals: `prompt_tokens_estimated` and a
fixed sentence that the estimate counts only the prompt; context the provider reads is extra and not estimated.

- R15. `preview` never starts a process, never creates a worktree and never writes a file.
- R16. `scaffold <strategy>` prints a valid plan skeleton for that strategy (placeholders for tasks, writers
  isolated in `parallel`, readers in `research`); `auto` prints the `economy` skeleton. With `--out <file>` it
  writes that file only if it does not exist. Every skeleton passes `validate` once its placeholders are text.

Run result (`run`, stdout, and `<out>/state.json` when `--out` is given): `schema: "downshift.plan.result/v1"`,
`run_id`, `status` (section 7), `started_at`, `ended_at`, `strategy_effective`, `usage` (R31), and per worker:
`id`, `provider`, `mode`, `isolate`, `status`, `reason`, `attempts[]` (each: `model`, `tier`, `status`, `reason`,
`exit_code`, `started_at`, `ended_at`, `duration_ms`, `timeout_s`, `reported_tokens` or null, `session_id` or
null, `worktree` or null, `patch` or null, `result` or null), `escalation` (section 8). `result` and `patch` are
paths under `--out`; without `--out` the final text is inlined as `final_text` on stdout only.

## 3. Strategies, provider choice and model choice

### 3.1 Strategy resolution

Precedence: `--strategy` flag, then the plan's `strategy`, then `auto`. There is no saved strategy (G7).

| Strategy | Max workers | Max concurrency | Rule | Default tier (R20; the planner's model wins) |
|---|--:|--:|---|---|
| `auto` | 16 | 4 | resolves to one of the rows below | of the resolved row |
| `economy` | 1 | 1 | one worker | `small` |
| `balanced` | 4 | 2 | any mix that passes R12 | `mid` |
| `quality` | 1 | 1 | one worker; never inferred | `frontier` |
| `research` | 16 | 4 | every worker `mode: read` | `mid` |
| `parallel` | 16 | 4 | at least two writers, every writer isolated | `mid` |

- R17a. `auto` resolves in this order, first match wins: one worker, `economy`; every worker `read`, `research`;
  two or more writers all isolated, `parallel`; otherwise `balanced`. `auto` never resolves to `quality`; a
  review pass must be asked for. The result records requested, effective and the rule that fired.
- R17b. More workers than the cap refuses the plan. `research` refuses a writer. `parallel` refuses a
  non-isolated writer. The effective concurrency is min(plan `concurrency` or cap, cap).

### 3.2 Provider choice per worker

Inputs: the worker's `provider`, the plan's `providers` order, which provider executables are installed
(`exec.LookPath` of the descriptor's executable name), and one quota snapshot taken at validation:
`quota.Load(id)` then `quota.Readings(snapshot, policy.QuotaTTL)` for each provider, assessed with
`headroom.Assess(providers, readings, now, policy.CriticalThreshold)`. Policy comes from `policy.Load` with the
user path and the project path of the plan's directory; a policy load error refuses the plan (doubt is not
silence for a CLI the user runs).

- R18. Rules, in order:
  1. Explicit provider not installed: refused.
  2. Explicit provider installed: kept, whatever its strain (O3). If it is critical, `provider_reason` is
     `explicit_critical` and the note's headroom verdict is `ignored`. Never switched to another provider.
  3. `auto`: candidates are `providers` in order, filtered to installed. None left: refused.
  4. `auto`: the first candidate whose strain is not `critical` wins (`ok` and unknown both pass: a missing
     reading lets work through). Reason `order` if it is the first candidate, else `headroom`.
  5. `auto` and every candidate critical: the first candidate wins, reason `all_critical`, a warning line is
     printed (O3). Never blocks, never forces a cheaper model.
- R19. The same plan with the same readings and the same `now` gives the same choice for every worker. `run`
  takes a fresh snapshot at start; `preview` and `run` may differ only if the readings changed, and the result
  shows the strain used.

There is no load balancing: every `auto` worker goes to the same provider when readings are equal (G5).

### 3.3 Provider descriptors

A descriptor is data in `internal/plan/provider`: executable name, argv template for `read` and `write`, how the
prompt is delivered (stdin), output format, which fields carry the final text, the session id and token usage,
whether reported input already includes cached tokens, whether `effort` is supported, and whether read-only is
enforced by the CLI or only by the contract. Shared code reads these fields; it never branches on a provider id.
Exact flags must be checked against the installed CLI's `--help` by frente P3 (G4).

| Fact | `claude-code` | `codex` |
|---|---|---|
| Non-interactive run, JSON output | print mode with JSON output | `exec` with JSON event lines |
| `read` mode | plan permission mode | read-only sandbox |
| `write` mode | accept-edits permission mode | workspace-write sandbox |
| Bypass flags | never emitted | never emitted |
| Reports tokens | yes | yes |

- R17c. `effort` reaches argv only when the descriptor supports it; otherwise the worker records
  `effort_ignored`. A descriptor never emits a permission-bypass flag; a test scans every template.

### 3.4 Model choice: the planner decides (owner decision O8)

The planner chooses the model and the effort of every worker in its plan. The runner applies them and does not
move them. There is no second router, no built-in tier table and no downgrade on the runner's side.

- R20. The worker's `model` is passed to the provider CLI verbatim when set. When it is not set, the CLI default
  runs, and the result says so. A `tier` in a worker (small, mid, frontier) is resolved through
  `catalog.ModelFor(provider, tier)`, and a catalog with no model for that pair refuses the plan.
- R21. The runner calls `decide.Decide` only to record the routing note and to honour the project policy the user
  declared. The planner's model is the request, so a rule or floor from `policy.json` may still change it, as for
  any spawn. Nothing else moves it: no risk veto (O8), no automatic upgrade after a failure.
- R22. The routing note is returned in `preview` and in the result, with only enums, tiers, numbers and times
  (the contract's invariant: no task text in a note).

## 4. Budget: calls, wall time and tokens

All four limits are optional; absent means unlimited. There is no money limit (O1).

- R23. **Admission.** Each attempt asks for admission when a concurrency slot frees and it is next in the queue
  (plan order; an escalation retry goes to the front, section 8). Checks run in this order, the first that fails
  decides the reason:
  1. `calls`: attempts admitted so far `>= calls` refuses (`budget_calls`).
  2. `wall_s`: `now - run start >= wall_s` refuses (`budget_wall`).
  3. `prompt_tokens`: estimated tokens of attempts admitted so far plus this one `> prompt_tokens` refuses
     (`budget_prompt_tokens`).
  4. `reported_tokens[provider]`: reported tokens of finished attempts of this provider `>= cap` refuses
     (`budget_reported_tokens`).
- R24. A refused attempt ends `skipped` with its reason: no process, no worktree, a result entry, counted as not
  succeeded.
- R25. An admitted attempt counts one call immediately, even if it then fails to start or fails. Its effective
  timeout is min(its `timeout_s`, wall time left), never below 1 s.
- R26. Prompt estimate: ceil(code points of task plus contract / 4), minimum 1, per attempt. Validation refuses
  the plan when the sum over first attempts exceeds `prompt_tokens`, before anything starts, in `validate`,
  `preview` and `run`.
- R27. **Missing values let work through.** A provider that reports no usage adds nothing to `reported_tokens`
  and never trips that cap. An unknown reading never holds a worker. This is the MVP rule.
- R28. Limits only stop new work. A running attempt is never cancelled because a budget ran out; it stops only at
  its own effective timeout. With concurrency above 1 a cap can be overshot by the attempts already running; the
  result says so in `usage.overshoot`.
- R29. Reported tokens of an attempt: the descriptor's explicit total if present; else input plus output plus the
  cache fields the descriptor lists, never adding a cached field the descriptor marks as included in input.
- R30. Budgets are per run. There is no per-project or per-day budget (G8).
- R31. `usage` block: `calls`, `wall_s`, `prompt_tokens_estimated`, `reported_tokens` as a map per provider with
  `null` for a provider that ran and reported nothing, and `overshoot`. Never a cross-provider sum.

## 5. Worktrees

- R32. **When.** Only for an attempt with `mode: write` and `isolate: true`, created at admission (a skipped
  attempt gets none). Each attempt gets a new worktree, including an escalation retry.
- R33. **How.** Worktrees live under `<repo root>/.downshift-worktrees/<slug>`, created with
  `git -C <repo> worktree add`. A read attempt is detached at the base commit (`--detach`, no branch). A write
  attempt gets an ephemeral branch `downshift/<run-id>/<slug>`, so its commits can be reviewed and merged later.
  The directory `.downshift-worktrees/` is ignored through `.git/info/exclude` when `.gitignore` does not already
  ignore it, so the user's `git status` stays clean. The slug follows the grammar of R54. Each attempt writes a
  lease file `.downshift-worktrees/.leases/<slug>.json` (write-to-temp then rename). An existing target is an error
  (`worktree_failed`, worker failed, run continues).
- R34. **Never removed automatically.** No code path of `run`, of a failure, of a timeout or of a cancel removes a
  worktree or runs `git worktree prune`.
- R35. **Patch.** Applying a change back is not part of `run`; it is R62. After an isolated attempt ends (any status but `skipped`), the runner marks new files with
  `git add --intent-to-add` inside the worktree and takes `git diff --binary HEAD`. With `--out`, a non-empty diff
  is written to `<out>/<worker-id>/<attempt>.patch`; an empty diff writes nothing and records `patch: null`.
  Without `--out` the patch is not written; the worktree holds the change. A patch from a failed attempt is
  marked `untrusted: true`. A non-isolated writer gets no patch (it would mix the user's own changes).
- R36. **Explicit clean.** `downshift plan clean [--run <run-id>]` lists, from `git worktree list --porcelain` of
  the current repository, the worktrees under `.downshift-worktrees/` (optionally of one run), with dirty or clean
  status and whether each has an ephemeral branch. It removes nothing without `--yes`. With `--yes` it removes clean
  worktrees and their ephemeral branches; dirty ones are skipped unless `--force` is also given. It never touches a
  path outside `.downshift-worktrees/`.
- R37. Paths are OS-agnostic: no `/` join, no `$HOME`, Windows tested in `test-os`.

## 6. Worker contract and mechanical acceptance

### 6.1 Fixed contract

- R38. Every attempt's prompt is the task followed by the Downshift worker contract, appended once, at the end,
  with no option to turn it off. The contract text is Downshift's own and states: (1) work only on the assigned
  unit and inside the declared scope; (2) do not delegate, spawn subagents or start another agent CLI; (3) do not
  commit, push, tag, release or open a pull request; (4) end with a short report of result, evidence, files
  changed, checks run, failures and risks; (5) the time budget in seconds, with the report written by 70% of it.
- R38a. The first line of the prompt states the budget in one sentence (time in seconds, the report by 70% of it),
  before the task; R38 still appends the full contract at the end. A cue at the end alone arrives too late for a
  long task (survey idea A3). The line carries numbers only, never task text.
- R39. The prompt reaches the process on stdin, never on argv and never through a shell. Shell metacharacters
  in a task are inert (tested).
- R40. Compliance is **not** checked. The runner does not inspect git history or the report's sections. The only
  mechanical protections are the descriptor's read or write mode and the absence of bypass flags (R17c). With
  `read` on a CLI that cannot enforce it, the result records `read_only: contract_only`.

### 6.2 Mechanical acceptance

- R41. Each finished attempt is classified, first match wins:
  1. deadline reached: `timed_out`, reason `timeout`;
  2. exit code not zero: `failed`, `exit_code`;
  3. output is not valid for the descriptor's format: `failed`, `invalid_output`;
  4. the descriptor's error marker is present in the last result object: `failed`, `provider_error`;
  5. no non-empty final text can be extracted: `failed`, `no_final_text`;
  6. otherwise `succeeded`.
  A spawn error (executable missing, start refused) is `failed`, `start_failed`; a worktree error is `failed`,
  `worktree_failed`. A panic while running one attempt is `failed`, `internal`, and the run continues.
- R41a. Verification is reported in three words, never implied (survey idea A4): `passed`, `failed`, `unchecked`.
  An attempt with no `verify_argv` is `unchecked`; a merge that changed nothing is reported `no-op`; the run totals
  count unchecked attempts on their own line, so a green run with nothing verified never reads as verified.
- R42. Final text extraction follows the descriptor's ordered list of fields; the session id, when present, is
  recorded (the MVP never resumes it). Raw stdout and stderr are never written to disk (they may echo the task,
  O4); they are parsed in memory and dropped.
- R43. Timeout and cancel kill the whole process tree in three steps: SIGINT to the group, wait 1.5 s, SIGTERM,
  wait 2 s, then SIGKILL. On Windows: Ctrl-Break, wait 1.5 s, then terminate the job object. The runner always
  imposes its own deadline, whatever the CLI does. (Timings from the business plan, section 5 of its
  orchestration spec.)

**Gap, stated in every output:** `succeeded` means "exited cleanly and left a final text". It does not mean the
task's definition of done was met. There is no acceptance by task criterion (tests, lint, expected files); the
lead checks that in the integrated checkout (G1).

## 7. Job states and recovery

### 7.1 States

| Unit | States | Terminal |
|---|---|---|
| Attempt | `queued`, `running`, `succeeded`, `failed`, `timed_out`, `skipped`, `cancelled` | the last five |
| Worker | status of its last attempt that ran; if the retry was skipped, the status of the first | |
| Run | `running`, then `completed`, `partial`, `failed`, `cancelled` | the last four |

- R44. Transitions: `queued` to `skipped` (admission refused) or `running` (admitted, worktree created if any,
  process started); `running` to a terminal of R41; `queued` or `running` to `cancelled` on interrupt. A terminal
  attempt never changes again.
- R45. Run status: `completed` when every worker `succeeded`; `partial` with at least one success and one
  non-success (skipped counts); `failed` with no success; `cancelled` on SIGINT, SIGTERM or Ctrl-Break, which
  kills every live tree (R43), marks queued attempts `cancelled` and starts nothing new.
- R46. Exit codes: 0 `completed`; 1 `partial` or `failed`; 2 usage or validation error; 130 `cancelled`.

### 7.2 Persistence and recovery

- R47. With `--out <dir>`, the directory must be new or empty (else refused before anything starts). The runner
  rewrites `<dir>/state.json` atomically (temp file in the same directory, then rename) after every attempt
  transition, and writes each final text to `<dir>/<worker-id>/<attempt>.md`. The state file contains no task
  text, no prompt and no stderr.
- R48. Without `--out`, the runner writes nothing but worktrees.

**What the MVP guarantees.**

- A reader of `state.json` always sees a complete earlier state, never a torn file.
- Every worktree survives any failure and is found by `plan clean` (R34, R36).
- A clean interrupt (R45) leaves no live child process and a final `cancelled` state.
- One failing, panicking or hanging worker never stalls the run (R41, R43).

**What the MVP does not guarantee.**

- No resume. A run killed hard (SIGKILL, power loss) leaves `state.json` at `running` and is lost (O4). Since the
  task text is never on disk, the run cannot be replayed from its state; the lead reruns the plan file.
- No reaping of children after a hard kill of the runner: on Unix the workers' process groups may outlive it
  (G2).
- No detection of a stale `running` state by another command.

## 8. Escalation (O5)

- R49. Off by default. On only with plan `escalate: true` or the `--escalate` flag (the flag can only turn it on).
- R50. Trigger: the first attempt of a worker ends with reason `exit_code`, `invalid_output`, `provider_error`,
  `no_final_text` or `timeout`. Never on `start_failed`, `worktree_failed`, `internal`, `skipped` or
  `cancelled` (a larger model does not fix them).
- R51. Target: one tier above the first attempt's effective tier, on the same provider, with the same mode, task
  and contract, at `catalog.ModelFor(provider, tier+1)`. No escalation (recorded as `escalation.reason`) when:
  the requested tier (R20; for a pinned model, the catalog tier of that id) is unknown; the first attempt already
  ran at the requested tier (`at_ceiling`); the target would be above the requested tier; the catalog has no
  model there or it is `explicit_only`. A pinned model therefore never escalates. One retry at most, no chain.
- R52. The retry is a new attempt: it goes to the front of the queue, asks for admission like any attempt and
  counts against calls, wall time, prompt tokens and reported tokens (R23). If it is refused, the worker keeps the
  first attempt's status and `escalation.reason` is the budget reason. An isolated retry gets a new worktree; the
  first attempt's worktree and patch stay, marked untrusted.

## 9. Behaviour tests

All tests run on deterministic fakes: an in-memory launcher for unit tests, and for end-to-end tests the test
binary re-executed as a fake provider (helper-process pattern) copied onto a temporary `PATH` as `claude` and
`codex` (with `.exe` on Windows). The clock is injected; no test sleeps on wall time. Tests isolate `HOME` and
`DOWNSHIFT_STATE_DIR`. The gate for every frente is `make check`.

| Req | Behaviour proven | Command |
|---|---|---|
| R5-R13 | table: minimal plan accepted; unknown field (plan, budget, worker), version 2, duplicate id by case, unsafe id, `task`+`task_file`, missing `cwd`, `isolate` on read, `model`+`tier`, `auto`+`model`, unsafe model, two writers one not isolated, out-of-range numbers each refused with the worker and field named; relative paths resolve from the plan dir; `concurrency` above cap lowered | `go test ./internal/plan/planfile -run TestValidate` |
| R11 | isolated worker outside a repository refused; inside a repo with no commit refused | `go test ./internal/plan/planfile -run TestIsolationNeedsRepo` |
| R10, O4 | a `task_file` is read but its text is in no output and no file | `go test ./internal/plan/planfile -run TestTaskTextNotPersisted` |
| R17a-b | `auto` table (one, all read, two isolated writers, mix) and never `quality`; caps; research refuses writer; parallel refuses non-isolated writer; concurrency lowers only; flag beats plan beats `auto` | `go test ./internal/plan/strategy -run TestResolve` |
| R18-R19 | explicit not installed refused; explicit critical kept with `ignored`; `auto` order, skip critical, unknown passes, all critical picks first with warning; same input same output | `go test ./internal/plan/assign -run TestProvider` |
| R20-R22 | pinned model kept; tier resolves via catalog; risk text keeps tier; decider lowers per policy rule; no catalog model means no `--model`; note has no text | `go test ./internal/plan/assign -run TestModel` |
| R17c, R39 | every descriptor template has no bypass flag; prompt on stdin; metacharacters inert; `effort` dropped where unsupported | `go test ./internal/plan/provider` |
| R23-R31 | prompt cap refuses the whole plan; call cap skips the next attempt with no process; wall cap skips late attempts and shortens the admitted timeout to the rest (floor 1 s); reported cap per provider; missing usage never trips; running attempt not cancelled at cap; usage map with `null`, no sum | `go test ./internal/plan/budget` |
| R29 | token sum with explicit total, without total, and without double counting an included cached field | `go test ./internal/plan/provider -run TestTokens` |
| R32-R37 | worktree under `.downshift-worktrees/`, detached for reads and on an ephemeral branch for writes; existing target fails the worker only; never removed after success, failure, timeout or cancel; patch includes new files; empty patch writes nothing; clean lists, removes only with `--yes`, skips dirty without `--force`, ignores outside paths | `go test ./internal/plan/worktree` |
| R38, R40 | contract appended once at the end on inline, file and retry paths; the five invariants present by constant; read on a contract-only CLI recorded | `go test ./internal/plan/worker -run TestContract` |
| R41-R43 | classification table in order; extraction order; session id recorded; raw output not on disk; timeout kills the tree; panic in one worker fails only it | `go test ./internal/plan/worker -run TestAccept` and `go test ./internal/proctree` |
| R44-R48 | completed, partial, failed, cancelled and exit codes; state rewritten atomically per transition; non-empty `--out` refused; no file without `--out` | `go test ./internal/plan/run` |
| R49-R52 | off by default; each trigger and non-trigger reason; one tier up on the same provider; `at_ceiling`; pinned never escalates; retry front of queue and counted; budget-refused retry keeps first status; new worktree for retry | `go test ./internal/plan/escalate ./internal/plan/run -run TestEscalat` |
| R1, R15, R16 | `preview` with a recording fake starts no process, creates no worktree, writes no file; every scaffold validates; `validate`/`preview`/`run` refuse the same plans; run with fake workers end to end | `go test ./cmd/downshift -run TestPlan` |
| layering | `internal/routing` never imports `internal/plan`; `internal/plan` never imports telemetry, accounting or an adapter | `go test ./internal/archtest -run TestPlanImports` |

## 10. Gaps

| Id | Gap | Owner |
|---|---|---|
| G1 | No acceptance by task criterion. `succeeded` is mechanical only, so O5 escalation never fires for a clean wrong answer. Option: an optional `accept` argv run in the worker's cwd after success, exit 0 required, counted as part of the attempt. | Tiago Vilas Boas (product decision) |
| G2 | Children may outlive a hard-killed runner on Unix; no reaping, no stale-state detection. | Backend Platform lead of P0/P8 |
| G3 | No resume of a run or of a provider session (O4 makes the run unrecoverable by design). | Tiago Vilas Boas, if O4 is ever revisited |
| G4 | Exact CLI flags, output event shapes, error markers and token fields of `claude` and `codex` must be verified on the installed versions; this spec names behaviour, not flags. | Implementer of P3 |
| G5 | No load balancing or quota spread across providers; all `auto` workers land on the same provider for equal readings. | Tiago Vilas Boas |
| G6 | No seed paths: an isolated writer starts from `HEAD` and does not see the user's uncommitted changes. | Tiago Vilas Boas |
| G7 | No persisted plan defaults (strategy, provider order, budget) in a user file; built-in defaults only. | Tiago Vilas Boas |
| G8 | Budgets are per run only; no per-project or per-day ceiling. | Tiago Vilas Boas |
| G9 | Worker tokens do not reach the stage 3 per-harness report; needs the stage 3 report shape first. | Stage 3 lead, then P10 |
| G10 | Worker-CLI facts live in a plan descriptor table, not in `core.HarnessCapabilities` (which today describes hook channels). The layering rule asks for harness facts in that struct; moving them is a core contract change. | Tiago Vilas Boas (architecture) |
| G11 | The prompt estimate (code points / 4) is a coarse heuristic and ignores context the provider reads. | AI / FinOps lead |
| G12 | Strategy default tiers (R20: small, mid, frontier) are this spec's choice, not an owner decision. | Tiago Vilas Boas |
| G13 | Estimate of 41 points (section 11, after the business plan additions P11 and P12) exceeds the MVP plan's 26 for stage 4. Cut candidates: `scaffold` (P9, 1), escalation (P7 and its run slice, 3). | Tiago Vilas Boas |

## 11. Implementation fronts

For implementers who know only this document and the Downshift contracts. Every front: one PR, at most 5 points,
copyright headers, no file over 300 lines, `make check` green, tests of section 9 for its requirements, no task
text on disk. At most three PRs open at once (waves below).

New packages, all under `internal/plan/` (screaming the domain): `planfile`, `strategy`, `assign`, `provider`,
`budget`, `worktree`, `worker`, `escalate`, `run`; plus `internal/proctree`. Dependency direction: `cmd/downshift`
wires catalog, quota and policy into `run`; `run` depends on the other plan packages through small interfaces;
`assign` depends on `routing/decide`, `routing/headroom`, `routing/policy`, `routing/contract`; nothing in
`internal/routing` or `internal/core` imports `internal/plan`.

| Front | Points | Files (owned) | Depends on | Gate |
|---|--:|---|---|---|
| P1a Types and inventory | 1 | `internal/plan/planfile/types.go` (Plan, Worker, Budget, Mode, Strategy, Status, Reason enums), `internal/archtest/plan_test.go`, `docs/contrib/coverage.md` (lines for every package above, added once here) | none | `make check`; `TestPlanImports` |
| P0 Process tree | 2 | `internal/proctree/*` (exported group start, polite then forced tree kill, Unix and Windows files); `internal/execwrap/procgroup_*.go` switched to call it, behaviour unchanged | none | `make check`, `test-os` green, existing execwrap tests unchanged |
| P5 Worktrees | 5 | `internal/plan/worktree/*` | P1a (Mode) | R32-R37 tests; `test-os` green |
| P1b Parse and validate | 3 | `internal/plan/planfile/{parse,validate,normalize}.go` and tests | P1a | R5-R13 tests |
| P4 Budget | 2 | `internal/plan/budget/*` (pure admission, injected clock) | P1a | R23-R31 tests |
| P7 Escalation rule | 1 | `internal/plan/escalate/*` (pure: first attempt, requested tier, catalog interface in, target or reason out) | P1a | R50-R51 tests |
| P2 Strategy | 2 | `internal/plan/strategy/*` (pure) | P1b | R17a-b tests |
| P3 Providers and acceptance | 5 | `internal/plan/provider/*` (descriptors, argv, output parse, tokens), `internal/plan/worker/{contract,accept}.go` | P1a | R17c, R29, R38-R42 tests; G4 verified on real CLIs, recorded in the PR |
| P6 Assign | 3 | `internal/plan/assign/*` (provider choice, model via `decide.Decide`) | P1b | R18-R22 tests |
| P8 Run | 5 | `internal/plan/worker/launch.go`, `internal/plan/run/*` (queue, slots, states, `state.json`, cancel, escalation wiring) | P0, P3, P4, P5, P6, P7 | R41-R52 tests; race detector |
| P9 CLI | 3 | `cmd/downshift/cmd_plan*.go`, `cmd/downshift/help_spec_plan.go`, one `case "plan"` in `cmd/downshift/main.go` | P2, P8 | R1, R15, R16 e2e; `./examples/run-all.sh` unchanged |
| P11 Events and rework ids | 3 | `internal/plan/events/*` (R59, zero-prompt fields, rework hashes) | P8 | R59 tests; no prompt or path in any written file |
| P12 Merge gate | 5 | `internal/plan/merge/*` (R61 and R62: dry run, verify under compact, CAS fast-forward, quarantine) | P5, P8 | R61-R62 tests on fake git and fake verify; `test-os` green |
| P10 Docs | 1 | `docs/reference/harness-matrix.md` (plan runner row: plain command, any harness), `CHANGELOG.md` | P9 | `make check` (docscheck) |
| **Total** | **41** | | | |

Waves (no two fronts in a wave share a file, a generated artifact or a pending decision):

1. P1a, P0, P5 in parallel (P5 uses only the `Mode` type; it can start on a stub and rebase onto P1a).
2. P1b, P4, P7 in parallel, after P1a merges.
3. P2, P3, P6 in parallel; P3 can start in wave 2 if a slot is free, since it needs only P1a.
4. P8 alone: it integrates every interface and owns the only scheduler.
5. P9, then P10, P11 and P12 in parallel (P12 needs only P5 and P8).

Decisions to settle before wave 3: G1 (changes P3, P8 and the schema if `accept` is added; adding a field later is
a schema change), G12 (default tiers, P6) and G13 (scope cut). G10 can wait: moving descriptor facts into
`core.HarnessCapabilities` later touches only P3's table.

## 12. Absorbed from the business plan (2026-10-27)

Source: `downshift-business-plan`, files `architecture/worktree-isolation-spec.md`,
`architecture/multi-vendor-orchestration-spec.md` and `architecture/domain-boundaries.md`. They are the owner's own
documents. Everything is tokens and calls only (O1), and no prompt or path is ever written to disk.

- R53. **Output ring buffer.** Each worker keeps stdout and stderr in memory only, in a circular buffer of 256 KiB
  each. Overflow drops the oldest bytes and counts them in `dropped_bytes`, shown in the result. Nothing from the
  buffer is written to disk (reinforces O4).
- R54. **Identifier grammar.** A worker id used in a path or a branch name matches `^[a-z0-9][a-z0-9._-]{0,63}$`.
  A leading `-`, `..`, a trailing `.lock`, `@{`, a control character or a path separator is refused at validation.
- R55. **Hermetic git.** Every git call is an argv, never a shell string, uses `--` before paths, and sets
  `GIT_TERMINAL_PROMPT=0` and `LC_ALL=C`. Read-only diagnostics (`status`, `diff --numstat`) also set
  `GIT_OPTIONAL_LOCKS=0`, so the runner never blocks an editor watching the repository.
- R56. **Symlink escape.** The repository root and the state directory are resolved with `filepath.EvalSymlinks`.
  Every path the runner writes or removes must be a strict descendant of `.downshift-worktrees/`; anything else is an
  error before any write.
- R57. **Read-only check.** After a read-only attempt, `git status --porcelain --untracked-files=all` in its
  worktree must be empty. A mutation marks the attempt `read_only_violated`, fails it, and keeps the worktree.
- R58. **Quarantine.** An attempt whose process died or failed and whose worktree has changes or commits is
  recorded as `quarantined` in `state.json`. `plan clean` lists it and never removes it without `--yes --force`.
  A clean abandoned worktree is removed only by `plan clean --yes`. No code path removes a worktree on its own
  (R34 stays).
- R59. **Zero-prompt events and rework.** The runner emits only these fields per worker event: task id (UUIDv7),
  tool-use hash (first 16 hex of SHA-256 of a per-run salt plus the raw id), harness, phase, duration in
  milliseconds and exit code. A scope hash is computed in memory from hashed relative paths of the diff and then
  dropped. An attempt is marked `reworked` when a later attempt names it as parent after a quarantine, or when
  its worktree is discarded and a new attempt touches the same scope hash within 15 minutes.
- R61. **Merge gate (explicit command).** `downshift plan merge <run-id> [--worker <id>]` applies one completed
  write attempt to the user's branch, fail-closed, in this order: the worker's diff stats are recorded; a three-way
  dry run with `git merge-tree --write-tree` must be clean, with no conflict; the verify argv set in the plan runs
  under `downshift compact -- <argv>` and must exit 0 within its timeout; then a compare-and-swap fast-forward of the
  target. If the target head moved, the dry run and verify repeat, up to 2 attempts, then the attempt is quarantined.
  The gate refuses when the user's working tree has uncommitted changes in the same files. It never forces.
- R62. **No automatic apply.** `run` never changes the user's branch. Only R61, run by the user, does. A failed or
  quarantined attempt is left in place for `apply`, `rework` or `discard`, decided by the user.
- R60. **Per-harness admission.** At most 2 running workers per provider by default, on top of the strategy cap
  of section 3.1. A provider in cool-down (strain above any percentage, R22) defers the attempt to the next
  provider in plan order, or parks it until the reset time, never failing it silently.

Tests for R53 to R62 are added to section 9 as rows, each on the fakes described there. The estimate grows by
8 points: new front **P11 Events and rework ids** (R59, 3 points, after P8, files `internal/plan/events/*`) and new
front **P12 Merge gate** (R61 and R62, 5 points, after P5 and P8). R53, R55 to R58 and R60 are folded into P5, P8 and P4.

New gaps:

| ID | Gap | Owner |
|---|---|---|
| G14 | **Resolved (owner, 2026-10-27): follow the business plan.** Hard cap 16 workers, default concurrency 4 (R13, strategy table). | Tiago Vilas Boas |
| G15 | **Resolved (owner, 2026-10-27): follow the business plan.** The merge gate exists as an explicit, fail-closed command (R61), never automatic (R62). | Tiago Vilas Boas |
| G16 | **Resolved (owner, 2026-10-27): follow the business plan.** Worktrees under `.downshift-worktrees/` with an ephemeral branch for writes (R33). | Tiago Vilas Boas |

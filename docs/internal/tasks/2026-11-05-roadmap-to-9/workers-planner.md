<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# WK: Planner, workers and gateway in the default flow

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Part of the [roadmap to 9](README.md). Read at ebfc304c.

## Problem

The planner, the MCP workers and the 9router gateway are shipped, but a fresh
install uses none of them:

- `DOWNSHIFT_WORKERS` unset means `workers.Unverified()` (`cmd/downshift/cmd_mcp.go:76`):
  every `delegate` fails with `ErrNoVerifiedMode`.
- Setup and doctor never mention workers (`doctor_mcp.go:14-25` only checks the
  server answers).
- `plan` always emits `delegate` steps with `mode: read` (`internal/plan/next.go:76`)
  and does not know whether workers are on (`internal/mcp/tools/info_plan.go:47-51`),
  so with workers off its next steps fail.
- `gateway on` succeeds while it affects nothing: its env reaches only delegate
  workers (`cmd_gateway_switch.go:18-24`).
- `plan` and `delegate` calls are not recorded, so adoption cannot be measured.

Flipping the default today would also expose three holes found in the code:

| Hole | Evidence |
|------|----------|
| Resume widens a read worker: a `send` or `steer` turn runs without plan mode, on the default model | `launch.go:105` replaces the args; `claudeResumeArgs` (`launch_agents.go:143-146`) has no `--permission-mode`, `--model`, `--effort`. Comments at `launch.go:86-88` and `:264-266` say the opposite. No test |
| An explicit `dir` is never checked to be inside the server root | `work_tools.go:213-221`, against the schema (`:29`) and `docs/features/safety.md:101` |
| A Codex worker can start workers (spec R26) | Full env inherited (`launch.go:129-131`); only Claude gets `--strict-mcp-config` (`launch_agents.go:84` vs `:187`); setup registers Downshift for Codex |

Other gaps: no total job cap per session (`queue.go:54-79`) and `delegate`
ignores strain; Windows kills only the parent process (`procgroup_windows.go`);
the agent-to-harness map is duplicated (`workers/view.go:115-137`,
`mcp_worker_tokens.go:17`); Codex read-only has never been observed refusing a
write (`docs/internal/plans/mcp-delegation-spec.md:180-188`); write workers get
the whole environment; spec R30's worktree prune command does not exist.

## Design

Option B of four considered (A: read on for all CLIs; C: setup asks only;
D: project key only):

- One shared resolver, `policy.ResolveWorkers`. Precedence:
  `DOWNSHIFT_WORKERS` (gains `off`) > user `policy.json` > project
  `policy.json` > default.
- **A project file can only lower the mode.** A project `write` is ignored with
  a note, so a cloned repo cannot raise its own permissions.
- The default becomes `read`, only for worker CLIs whose caps record an
  observed refused write (Claude today, not Codex), and not on Windows until
  WK-06 merges. Write stays opt-in.
- Setup asks interactively. `plan` receives the resolved state.
- **Layering.** One MCP server serves every harness with an MCP client.
  Worker-CLI facts become data in `HarnessCapabilities`, read in
  `cmd/downshift` and passed in; `internal/workers` does not import `core`;
  adapters do not change. Harnesses without an MCP client see the computed
  state in `doctor` and `report`, recorded in `harness-matrix.md`.
- The planner never moves a native part to a worker. The orchestrator chooses
  (O8).

### Adoption telemetry without prompts

- A new event: `Outcome = "mcp_tool"`, `Agent` = tool name (`plan`,
  `delegate`), `ErrorCode` = refusal code (`workers_off`, `write_off`, `cap`,
  `budget`, `strain`, `dir`).
- An int-only `ToolCounts` block: `steps`, `delegate_steps`, `spawn_steps` for
  `plan`; `tasks`, `write`, `gateway_routed` for `delegate`. No task text, dir
  or file names.
- Excluded from routed-spawn accounting, like `worker_ended`
  (`internal/telemetry/aggregate.go:22-23`).
- A `report` and `stats` line, for example:
  `Planner: 6 plans, 14 steps (9 spawn · 5 delegate) · Workers: 5 delegated, 4 done, 1 refused (workers_off) · gateway 0/5`.
- It ships first, so a baseline exists before the flip.

## Tasks

| ID | Title | Pts | Depends on | Phase |
|----|-------|-----|------------|-------|
| WK-01 | Resume keeps mode, model and effort | 3 | none | 0 |
| WK-02 | Explicit `dir` must stay inside the server root | 2 | none | 0 |
| WK-03 | Recursion guard for every worker (R26) | 3 | none | 0 |
| WK-04 | Session job cap and strain-aware `delegate` | 3 | none | 0 |
| WK-06 | Windows kills the whole worker tree; worker tests on Windows | 3 | none | 0 |
| WK-07 | Worker-CLI facts as data in `HarnessCapabilities` | 2 | none | 0 |
| WK-12a | Adoption event and recording | 3 | none | 0 |
| WK-12b | Report and stats line | 2 | WK-12a | 0 |
| WK-08 | Shared worker-mode resolver (default still off) | 3 | WK-03, WK-07 | 1 |
| WK-09 | Serve, instructions and `status` use the resolver | 2 | WK-08 | 1 |
| WK-10 | `plan` next steps work with workers on or off | 3 | WK-08 | 1 |
| WK-11 | Doctor worker row and idle-gateway warning | 2 | WK-08 | 1 |
| WK-15 | Codex read-only evidence and a bypass-stays-in-worktree run | 2 | WK-01, WK-03 | 2 |
| WK-14 | Setup asks about workers | 3 | WK-08 | 2 |
| WK-13 | Read workers on by default for verified CLIs | 2 | phases 0 and 1 | 2 |
| WK-16 | Docs (separate PR) | 2 | WK-13, WK-14 | 2 |
| WK-05 | Strip secret-like env vars from write workers | 3 | WK-08 | 3 |
| WK-17 | `downshift workers worktrees` list and prune (R30) | 3 | none | 3 |

### WK-01: Resume keeps mode, model and effort (3)

- **Design:** the resume builder takes the original spec and recipe mode.
  Claude resume adds `--permission-mode <mode>` and the tuning flags; Codex
  resume uses the original sandbox (today `launch_agents.go:218-220` forces
  write resumes to read-only). One function per CLI, next to its launcher.
- **Files:** `internal/workers/launch.go`, `launch_agents.go`, `resume_test.go`.
- **Tests:** table over read/write × claude/codex: resume argv carries the same
  mode, sandbox, model and effort. E2E: `send` asks a read worker to write a
  file; the file does not appear.
- **Acceptance:** no resumed turn is wider or on another model than its first;
  the wrong comments are fixed.

### WK-02: dir containment (2)

- **Design:** resolve `dir` with `resolvedDir` (EvalSymlinks) and require
  `filepath.Rel(Root, dir)` with no `..`, for read and write.
- **Files:** `internal/mcp/tools/work_tools.go`, `cmd/downshift/cmd_mcp.go`,
  `work_tools_test.go`.
- **Tests:** outside dir, escaping symlink, `..`, Windows drive letters, an
  allowed subdirectory.
- **Acceptance:** an outside dir is refused with a fix line.

### WK-03: Recursion guard (3)

- **Design:** shared `startProcess` adds `DOWNSHIFT_WORKER=1` to every worker
  env; `serveMcp` resolves workers to `off` when it is set. The Codex worker
  gets an MCP-servers override (`-c mcp_servers={}` or equivalent), **verified
  against the CLI first**.
- **Tests:** marker present in the spec env; a server with the marker reports
  `off`. E2E: a Codex worker's `delegate` is refused.
- **Acceptance:** no worker can start a worker, on any route.

### WK-04: Job cap and strain (3)

- **Design:** `orchestration.max_workers_per_session`, default 32 (an
  assumption, tuned with WK-12 data). Under rate-limited strain, one call's
  tasks shrink to `ShrinkForStrain` (`policy/parallel.go:65`) with a refusal
  code. It never upshifts.
- **Files:** `internal/workers/queue.go`, `work_tools.go`, new
  `internal/routing/policy/workers_cap.go`.
- **Acceptance:** one session never starts more workers than the cap.

### WK-06: Windows process tree (3)

- **Design:** a Job Object with `KILL_ON_JOB_CLOSE` behind the `_windows` tag;
  add `./internal/workers/` to `test-os` in `ci.yml` and `ci-full.yml`.
- **Acceptance:** cancel and timeout leave no orphans on windows-latest.

### WK-07: Caps data (2)

- **Design:** caps fields `WorkerAgent string` and
  `WorkerReadOnlyVerified bool` (true for claude-code from the M4 evidence,
  `docs/evidence/mcp-claude-code-2026-10-31.md:18`; false for codex).
  `cmd/downshift` builds the maps from `core.AllHarnessCaps()`.
- **Acceptance:** one source for worker-agent facts; `archtest` green.

### WK-12a and WK-12b: Adoption telemetry (3 + 2)

- **Design:** as above. The edge code builds the event through an `OnCall`
  callback in `Deps`, so `tools` does not import `telemetry`.
- **Files:** new `internal/telemetry/mcp_tool.go`,
  `internal/accounting/workers.go`, `internal/mcp/tools/tools.go`,
  `info_plan.go`, `work_tools.go`; 12b: `cmd/downshift/report.go`,
  `cmd_stats.go`, `testdata/plain/report*.golden`.
- **Tests:** the event JSON contains no task text; aggregates exclude it from
  spawn counts.
- **Acceptance:** `report` shows the planner and worker line for 7 days.

### WK-08: Resolver (3)

- **Design:** `policy.ResolveWorkers(env, Locations, verified []string, goos)`
  returns `Workers{Mode, Agents, Source, Notes}`. Default constant stays `off`
  here. The `DOWNSHIFT_WORKER` marker forces `off`.
- **Files:** new `internal/routing/policy/workers.go`, `workers_test.go`,
  `schema.go`, `validate.go`.
- **Tests:** every precedence combination, a project `write` ignored with a
  note, invalid values, the marker.
- **Acceptance:** `downshift policy check` validates the key.

### WK-09: Serve uses the resolver (2)

- **Design:** `workerLaunchers(Workers)` builds routes only for `Agents`; the
  instructions follow the mode, and the off text says how to turn workers on;
  the `status` tool reports `workers.mode`, `source`, `notes`.
- **Tests:** instructions per mode stay under 4096 characters.

### WK-10: Plan with workers on or off (3)

- **Design:** `plan.Resolve` takes `Workers` like it takes `Gateway`. A worker
  part whose agent is off becomes a native spawn step with a warning; a write
  part gets `mode: write` only when write is on; the route says gateway only for
  a real delegate.
- **Files:** `internal/plan/plan.go`, `fill.go`, `next.go`, `next_test.go`,
  `info_plan.go`, `cmd/downshift/cmd_plan.go`.
- **Tests:** off/read/write × native/worker part; a `downshift plan` golden per
  mode.
- **Acceptance:** with workers off, no step calls `delegate`.

### WK-11: Doctor row (2)

- **Design:** `workers: read (claude) · write off · source default` with a fix;
  `gateway on` and doctor warn when the gateway is on and workers are off.
- **Files:** new `cmd/downshift/doctor_workers.go`, `doctor.go`,
  `cmd_gateway_switch.go`.
- **Acceptance:** `doctor --json` carries `workers` with a `fix`.

### WK-15: Evidence runs (2)

- **Design:** real runs in a throwaway repo: a Codex read worker asked to write,
  and a bypass worker checked to stay in its worktree. Recorded in
  `docs/evidence/mcp-workers-r27-<date>.md`; on a refused write, flip codex's
  `WorkerReadOnlyVerified`.
- **Risk:** the Codex result may stay negative; then the default excludes it.

### WK-14: Setup asks (3)

- **Design:** the interactive wizard asks off / read (default) / write;
  `--workers off|read|write` sets it; the answer goes to the user
  `policy.json`; `--dry-run` shows the diff; `--yes` without the flag writes
  nothing. The wizard is drawn as cards.
- **Files:** new `cmd/downshift/setup_workers.go`, `setup.go`, `setup_test.go`,
  `docs/guides/install.md`.
- **Acceptance:** write is never offered as a default.

### WK-13: The flip (2)

- **Design:** the default constant becomes `read`, limited to verified agents,
  `off` on Windows until WK-06 merges. CHANGELOG entry.
- **Tests:** default per OS and agent; a fresh HOME then `mcp serve`: Claude
  read delegates, Codex is refused with a reason.
- **Acceptance:** a fresh install delegates read workers with no env, and the
  WK-12 line shows the usage.

### WK-16: Docs (2)

- `safety.md:100`, `README.md:525`, `harness-matrix.md:26`, `plan.md:61`,
  `config.md`, `internal/limits/registry.go:235`. No doc says workers need the
  env.

### WK-05: Env strip for write workers (3)

- **Design:** `orchestration.worker_env_drop`, defaults `AWS_*`, `GH_TOKEN`,
  `GITHUB_TOKEN`, `NPM_TOKEN`, `*_SECRET*`, `*_PASSWORD*`; the CLI's own auth
  variable is always kept; case-insensitive on Windows; write workers only.
- **Risk:** a tool that needs a dropped var breaks; the key overrides it.

### WK-17: Worktree prune (3)

- **Design:** list and remove only marked `.downshift/worktrees/*`, with a
  confirmation; a worktree with unmerged commits needs `--force`.
- **Files:** new `cmd/downshift/cmd_workers_worktrees.go`,
  `internal/workers/worktrees.go`.

## Owner decisions

- **WK-Q1 (WK-13):** should `delegate` with `mode: read` be on by default, with
  no env set, for worker CLIs with an observed refused write (Claude today), on
  macOS and Linux, with Windows after WK-06?
- **WK-Q2 (WK-14):** should interactive setup ask off/read/write and save it in
  `~/.downshift/policy.json`, with `--yes` writing nothing?
- **WK-Q3 (WK-08):** may a project `policy.json` only lower the mode, so write
  comes only from the user file or `DOWNSHIFT_WORKERS=bypass`?
- **WK-Q4 (WK-05):** should write workers start without secret-like env vars by
  default, keeping the CLI's own auth variable, with a policy key to override?

## Open gaps

- The Codex MCP override flag (WK-03) is unverified.
- The cap of 32 (WK-04) is an assumption until WK-12 data.
- Whether `claude --resume` keeps plan mode without the flag is unverified;
  WK-01 removes the question.
- The `ErrNoVerifiedMode` path with workers off was taken from
  `internal/limits/registry.go:235`, not traced through the queue.

<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Plan: worker continuation for the MCP (`send`, `steer`, `answer`, `release`)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Owner decision 2026-10-31: break the three-week feature freeze
(`proof-of-value-roadmap.md`, P-freeze) for this one piece of work. The alethe
project (AGPL, read-only study; no code copied) already runs long-lived workers
you can send more work to, steer mid-flight, answer and release. This plan brings
that behaviour to Downshift's MCP without weakening the current isolation.

Scope here is **continuation only**: `send`, `steer`, `answer`, `release`. The
sibling streams that add `diff` and worker `status` are out of this plan.

## 1. The worker cycle today (read before the design)

- A worker is **one CLI process that runs once and exits**. `delegate`
  (`internal/mcp/tools/work_tools.go`) turns each task into a `workers.Spec` and
  calls `Queue.Delegate`; the queue assigns an id (`w001`...), starts up to its
  limit, and returns the ids at once.
- The prompt is fed on **stdin as text** and the process runs headless:
  `claude -p --output-format json --permission-mode plan|bypassPermissions
  --no-session-persistence --strict-mcp-config`, or `codex exec --json
  --ephemeral --sandbox read-only|workspace-write` (`launch_agents.go`). Both
  recipes are deliberately **non-resumable**: `--no-session-persistence` and
  `--ephemeral` keep nothing on disk.
- State lives only in memory in `Queue` (`queue.go`): a `job{id, spec, state,
  proc, timer}` under one lock. States are `queued → running → done/failed/
  cancelled/timeout`; `Finished()` is any state but queued/running. The task
  text is held in memory and never written to disk (spec R6).
- A finished worker's output becomes a **`Delivery`** (`deliveries.go`), kept in
  sequence until acknowledged. `collect` reads unacked deliveries, returns `ack`
  and `active`, and blocks up to 45s with `wait`.
- `cancel` finds a job by id in the `jobs` map, kills the process group
  (`killGroup`) if running, settles it `cancelled`, and `pump`s the next queued
  job. `Close` cancels everything so no worker outlives the session.
- So `collect`/`cancel` find a worker **by its id in a map, while its process is
  alive or already settled**. There is no conversation to resume: once a worker
  exits, only its `Delivery` remains. This is the gap continuation must fill.

## 2. Spike: live proof (claude 2.1.314, codex 0.163.0)

Run in a throwaway git repo, `--permission-mode plan`, model `haiku`, trivial
prompts. Raw output kept outside the repo (`$KIROCREW_SCRATCH/spike-send/
evidence/`), not committed. The claude session artifacts the spike created were
deleted afterwards.

**(a) `send` — session resume works and saves re-reading.** A first
`claude -p --output-format json` **without** `--no-session-persistence` returned
a `session_id` and stored a codeword. A second `claude -p --resume <id>` on the
same id answered the codeword correctly — same conversation, same `session_id`
(no fork). Token usage showed the win: `input_tokens` stayed at 2 on both turns
while `cache_read_input_tokens` grew (11,382 → 35,805). The prompt history is
not re-sent as fresh input; it is served from the prompt cache. That is the
saving `send` buys: continue a worker without re-paying to re-read its context.

**(b) `steer` — stream-json keeps one process and serializes turns.** With
`claude -p --input-format stream-json --output-format stream-json
--replay-user-messages`, the process stays alive and reads user messages off
stdin. Sending a second user message during the run produced **two `result`
events in order**: the first turn finished its task, then the second turn ran as
a new turn on the same conversation. The second message was **neither ignored
nor dropped — it was enqueued as the next turn**. This is exactly the mechanism
`send` needs on a live process, and confirms claude has **no native mid-turn
steer**: to interrupt, you send `control_request {subtype: interrupt}` and
re-deliver the message as the next turn (alethe does this; it uses codex's
native `turn/steer` only for codex).

**(c) Codex — resume exists (help only, no paid turn).** `codex exec resume
<SESSION_ID|--last> [PROMPT]` and `codex exec fork` exist. But the current
worker recipe passes `--ephemeral` ("run without persisting session files to
disk"), so a worker today is **not resumable**; `send`/`steer` for codex require
dropping `--ephemeral` and capturing the session id from the `--json` events.

### Spike findings in brief

1. `send` is real: `--resume <session_id>` continues the same conversation.
2. The saving is in the cache — history is not re-sent (input stayed 2 tokens;
   cache_read grew), so a follow-up turn is far cheaper than re-delegating.
3. stream-json keeps the process alive and **enqueues** a second user message as
   the next turn (it does not interrupt the running one).
4. claude has **no mid-turn steer**; interrupt + re-deliver is the only path.
   codex has native `turn/steer` and `codex exec resume`.
5. Enabling any of this means **dropping `--no-session-persistence`
   (claude) / `--ephemeral` (codex)**, which writes a transcript to disk — the
   isolation trade-off below.
6. Codex resume was read from `--help` only; no paid codex turn was run.

## 3. Proposed design

Principle held throughout: **simple, agnostic, the worker never climbs a tier,
isolation never gets worse**. A continuation never changes `model`/`effort`/
`mode`; it reuses exactly what `delegate` started the worker with.

### `send` — accept

More work for an existing worker, same conversation. Shape:
`send(job_id, task)`.

- **Worker still running** → enqueue the task as the worker's next turn (an
  in-memory `inbox` on the job). No interruption, no polling. This matches the
  stream-json behaviour proved in 2(b).
- **Worker already settled** → resume its session (`--resume <session_id>` for
  claude, `codex exec resume <id>` for codex) with the new task, reusing the
  original spec's `model`/`effort`/`dir`/`mode`. The result comes back through
  `collect` as a new delivery on the same job id.
- The queue gains, per job, a stored `session_id` (captured from the first
  turn's output) and an `inbox []string`. States need `blocked` (worker waiting
  on a question) added; `Finished()` stays as is but a settled job is now
  *resumable*, not terminal.
- Requires the launch recipe to persist the session (drop
  `--no-session-persistence` / `--ephemeral`) and to capture the session id.

### `steer` — accept (behaviour differs by agent)

Correct a worker mid-turn without killing it. Shape: `steer(job_id, message)`.

- **codex**: native `turn/steer` on the running turn.
- **claude**: no native steer. Enqueue the message at the **front** of the
  inbox, then send an interrupt to the running turn; when it aborts, the inbox
  is handed back and the steer runs as the next turn (the alethe approach, which
  follows directly from spike 2(b)).
- If no turn is running, `steer` degrades to a front-of-queue `send`.

### `answer` — accept, but only under write/bypass

Answer a permission request a worker stopped on. Shape:
`answer(job_id, decision)` with `decision ∈ {accept, accept_session, decline,
abort}`.

- This is only reachable when the worker can *ask* — i.e. an interactive
  approval channel exists. The read-only recipe (`--permission-mode plan`,
  codex `--sandbox read-only`) has **no approval channel**: a tool needing
  permission is auto-denied, never paused. So `answer` is meaningful **only for
  write/bypass workers that run with an approval policy**, which Downshift does
  not run today (write workers use `bypassPermissions` / `-a never`, which also
  never ask).
- **Recommendation: defer `answer`** until a "write with approval" worker mode
  exists. Shipping it now would be a tool with nothing to answer. Keep the
  `blocked` state and the plumbing so it is a small follow-up when that mode
  lands.

### `release` — accept

Let go of a settled worker you have no more work for. Shape:
`release(job_ids)`.

- Once `send` can revive a settled worker, a settled job is no longer free to
  forget: its `session_id` transcript is on disk and (for a live-process model)
  its process may be parked. `release` is the explicit end-of-life: drop the
  in-memory job record **and delete its on-disk session transcript** (the
  isolation cleanup in §4). `Close`/`cancel` must do the same deletion for
  workers they settle.
- Without `release`, resumable sessions would accumulate transcripts on disk.
  It is the counterpart that keeps the "nothing lingers" property.

## 4. Risks: isolation, security, concurrency, crashes

- **Transcript on disk.** Persisting the session writes a transcript. Measured:
  claude writes `~/.claude/projects/<dir-encoded>/<session_id>.jsonl` (+ a
  `session-env/<id>/` dir), **keyed by the working directory**. Because workers
  run in a **worktree**, not the user's project dir, the file lands in a
  *separate* project folder, so the user's `/resume` **in their own directory
  does not list it**. But a `/resume` run inside the worktree, or a global
  scan, would see it. Mitigation: `release`/`cancel`/`Close` **delete the
  worker's transcript and session-env**; prefer a worktree path the user never
  opens interactively. codex similarly persists under `~/.codex/sessions` unless
  `--ephemeral`.
- **Worker's session leaking into the user's resume list.** Follows from the
  above: contain it by the worktree-keyed path + deletion on release. Never
  resume the *user's* sessions from a worker.
- **`--strict-mcp-config` and the write refusals must not change.** Continuation
  reuses the original spec, so `--strict-mcp-config`, worktree-only writes and
  the broad-root refusals (`workerDir`) all still apply unchanged. A `send`/
  `steer` may not widen `dir` or flip `mode`.
- **Concurrency: `send` during a turn.** Enqueue, don't interrupt (proved
  safe in 2(b)). The inbox is guarded by the queue lock like every other job
  field. `steer` is the only path that interrupts, and only on explicit intent.
- **Process death.** If a live worker process dies between turns, a queued
  `send` must fall back to `--resume` of the stored session (alethe does this:
  "process gone, thread on disk, start it again"). If the session id was never
  captured (crash on turn 1), `send` fails with a clear error and the job stays
  settled — no silent new conversation.
- **Budget.** Each resumed/continued turn needs its own time budget, same rule
  as `delegate` (`ErrNoBudget`). A parked live process still holds a slot, so
  `release` (and a cap on parked workers) bounds resource use.

## 5. Tasks

Cap 5 points per task. P (parallel) share no files/decision; S (serial) name
the dependency.

| # | Task | Points | Depends on | Parallel |
|---|------|--------|------------|----------|
| T1 | Launch recipe: add a resumable claude/codex launch (drop `--no-session-persistence` / `--ephemeral`), capture `session_id` from output, keep `--strict-mcp-config` and all write refusals. Unit test the argv and the id capture. | 5 | — | P (own files: `launch_agents.go`, `launch.go`) |
| T2 | Queue state: add `session_id` and `inbox` to `job`, add the `blocked` state, and a resume-vs-live decision in `start`/`pump`. Tests for enqueue-while-running and revive-after-settle. | 5 | T1 | S (needs the id capture) |
| T3 | `send` tool + handler: schema, `inbox` enqueue when running, `--resume` when settled, new delivery on the same job id. Tool tests. | 3 | T2 | S |
| T4 | `steer` tool + handler: codex native steer; claude interrupt + front-of-inbox re-deliver; degrade to front `send` when idle. Tests per agent. | 5 | T2 | P with T3 (different tool, shared queue API from T2) |
| T5 | `release` tool + handler, and transcript/session-env **deletion** wired into `release`/`cancel`/`Close`. Test that a released worker leaves no on-disk session. | 3 | T2 | P with T3/T4 (own cleanup path) |
| T6 | Docs: update `mcp-delegation-spec.md` and the MCP tool descriptions for the three new tools; note `answer` deferred. | 2 | T3,T4,T5 | S (describes shipped behaviour) |
| T7 | (deferred) `answer` + a write-with-approval worker mode. Not in this plan; `blocked` state from T2 is the hook. | 5 | a new write-approval mode | — |

Order: **T1 → T2**, then **T3, T4, T5 in parallel**, then **T6**. T7 is out of
this plan. Each task is one reviewable PR (repo's ~400-line norm); T1/T2/T4 are
the heavy ones and stay within the 5-point cap by splitting tests from recipe if
either grows past it.

## 6. What this plan does not do

- No `answer` tool yet (nothing to answer under the current read-only / bypass
  recipes).
- No `diff` and no worker `status` (sibling streams own those).
- No model or tier change on continuation; a worker never climbs a tier.
- No price or dollar figures (product uses none).

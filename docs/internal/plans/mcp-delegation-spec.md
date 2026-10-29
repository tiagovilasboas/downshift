<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# MCP delegation server (`downshift mcp serve`)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Status: **Draft, 2026-10-28.** Reopens owner decision O7 for one part: the MCP server is Downshift's own, run by the
same binary, not a separate product. Names follow the business plan repository (`architecture/multi-vendor-orchestration-spec.md`, section 2): `downshift mcp serve` and tools named by their verb, under the server name `downshift` (the model sees `mcp__downshift__<verb>`). Decision O8 (the LLM chooses the model) is the premise. Clean-room: the behaviour
of the Alethe MCP path (kept outside this repository, no code copied) informs the shape; the choices below are ours.

## 0. What it is

`downshift mcp serve` is a stdio MCP server started by the orchestrator's harness (Claude Code, Codex). The orchestrator
calls its tools to delegate a worker with an agent CLI, choosing the **provider, model and effort** itself. The server
applies the choice, enforces the project policy the user declared, runs the worker in the requested mode, and returns
the result with a routing note. It never chooses a model on its own.

## 1. Requirements

- R1. One binary, JSON-RPC 2.0 over stdio, on the official Go SDK of the protocol, behind a port (decision of the
  K3 spike, revised the same day). The first reading of the spike chose a server of our own on the standard library,
  because the measured weight of the libraries was high. It had not measured protocol coverage. The protocol changed
  in revision 2026-07-28 (see section 3): there is no `initialize` handshake any more, every request carries its
  version and capabilities in `_meta`, `server/discover` is mandatory, results carry `resultType`, and the older
  handshake-based revisions (2025-11-25 and earlier) must still be served. A test of the SDK showed one server
  answering both eras, so the choice flipped: keeping up with a moving protocol is the cost we do not want to own.

  | Candidate | Licence (read in the repository) | Binary | Modules in the graph | Eras served |
  |---|---|--:|--:|---|
  | Official SDK v1.8.0 | Apache-2.0, MIT and CC-BY-4.0 (new contributions, older contributions, docs) | 10.6 MB | 9 | both (tested here) |
  | mark3labs/mcp-go v1.4.0 | MIT | 7.9 MB | 7 | not tested |
  | Own server | Apache-2.0 | about +0.1 MB | 0 | would need both implemented and kept current |

  How it is contained: the tools are plain functions (`internal/mcp/tools`); one package, `internal/mcp/transport`,
  imports the SDK and adapts them; `TestInternalPackagesUseOnlyTheStandardLibrary` allows the SDK in that package and
  nowhere else under `internal/`, and `TestExternalModulesStayAtTheEdge` keeps any other internal package from
  importing the transport (only `cmd/` does). The version is pinned in `go.mod`, `govulncheck` reports nothing for
  the MCP packages, and the notice is in `NOTICE`. Open: shipping the licence texts of the SDK and its modules in the
  release archives. Because the tools sit behind the port, replacing the SDK later means rewriting one file. The
  reference app's code is never used (AGPL-3.0, behaviour only).
- R2. Subcommand `downshift mcp serve`; no daemon, no listener, no network socket (O7 non-goal still holds: no app, no proxy).
- R3. Tools: `delegate` (harness, model, effort, mode `read|write`, prompt, cwd, optional `playbook`,
  optional `verify_argv`, optional `slug`; `kind` and `effort_class` are hints only), `status` (queue,
  phase per task, strain per quota window), `steer` (interrupt or steer a running task),
  `result` and `cancel` (task id), `providers` (installed agent CLIs and their models
  from the catalog), `playbook` (no name: the valid names; a name: that playbook's text).
- R10. `delegate` with a `playbook` name prefixes the worker's first message with
  `playbook.Block(sets, name)`: General plus the named set, delimited. An unknown name rejects the call with the
  valid names and starts no worker. Follow-up messages repeat nothing. The playbooks are read from the state
  directory at each call, so an edit in `downshift playbook` applies to the next delegation.
- R4. The model and effort in `delegate` are applied verbatim. The server passes them through the routing decider only
  to record the note and to honour `policy.json` (O8). No built-in table, no automatic upgrade, no risk veto.
- R5. Mode `read` runs the provider with its read-only permission mode; mode `write` runs in an isolated git worktree
  (stage 4 P5). The server never emits a permission-bypass flag.
- R6. The prompt is held in memory for the life of the job and never written to disk (O4). Output goes only where the
  caller names; an interrupted job is lost (O4).
- R7. Results carry enums, tiers, models, tokens and times, and the routing note. No task text in any note (contract).
- R8. Budget: the same call, wall-time and token caps as the plan runner (stage 4 section 4).
- R9. Errors are returned as tool errors, never as a crash; a malformed request is refused with a reason.
- R13. Every successful tool result carries a `strain` block: the fullest window per provider and its headroom name
  (`ok`, `strained`, `critical`, or unknown). It is advice for the orchestrator, never a reroute, and it is
  computed at call time. A host cannot push to the model, so the reading rides on the answer it already reads.
  (Survey idea A1.)
- R14. `result` is repeatable and acknowledged. Each delivery has a sequence number; the caller acks it
  and may ask for the same delivery again with `repeat`. A response lost to the host's call time cap or a dropped
  message loses nothing, and a repeat returns the same bytes. Results live in memory only (R6). (Survey idea A2.)
- R15. An unrecognised `mode` is read-only. The bypass is never a value the tool accepts. (Survey idea A5.)
- R16. The result and the routing note carry the playbook name used, or none, and never its text; "wrong rules or
  ignored rules?" is answerable from the record. (Survey idea A9.)
- R17. Dispatch obeys the plan runner's per-provider cap (plan runner spec R13a): a strained provider gets one
  attempt at a time. (Survey idea A10.)
- R11. Providers are started with an argument list, never through a shell, and `cwd` must be an existing directory the
  caller already has access to; a path that resolves outside the project root is refused. (AppSec review O8, T4.)
- R12. Playbook files that are not regular files (symlinks) are refused when read (`internal/playbook`, done).

## 2. Fronts

Status 2026-10-28: M1, M2a, M3 and M4 (Claude Code and Codex) are implemented and reviewed (AppSec: pass with conditions; QA run). M2b (the worker tools) waits for the plan runner. A failed tool result carries no `strain` block: advice rides on successful answers only.

The MCP is part of the MVP (owner decision O9, 2026-10-28), sliced: M1, M2a, M3 and M4 are stage 2b; M2b comes with the plan runner (stage 4).

| Front | Points | Scope | Depends on | Gate |
|---|--:|---|---|---|
| M1 JSON-RPC stdio core (MVP, K3) | 3 | `internal/mcp/transport`: the SDK adapter serving both eras (R1, R9) | none | unit tests on framed messages; `make check` |
| M2a Information tools (MVP, K4) | 3 | `internal/mcp/tools/*`: `providers` (detected models), `playbook`, `status`, `apply`; the strain block (R13) on every result | M1, K1, K2 | tool tests; no prompt on disk |
| M2b Worker tools (stage 4) | 3 | `delegate`, `steer`, `result`, `cancel` with R10, R14 to R17, over the plan runner's jobs | M2a, P8 | R3, R6, R7, R14 tests |
| M3 Command (MVP, K5) | 1 | `cmd/downshift/cmd_mcp.go`, `case "mcp"` in `main.go` (`serve`), help spec | M2 | e2e: initialize + tools/list over pipes |
| M4 Setup (MVP, K6) | 2 | `setup` registers the server in the harness's MCP config (opt-in, dry-run, undo by marker) | M3, S1 | setup tests; undo leaves other servers |
| M5 Playbook | 1 | `downshift-orchestrator` skill names the MCP tools (after M2) | M2, R2 | `TestSetupInstallsTheOrchestratorPlaybook` |

Dependency direction: `internal/mcp` depends on `internal/plan` through small interfaces; nothing in `internal/routing`
or `internal/core` imports `internal/mcp`. Each front is one PR of at most 5 points, `make check` green.

## 3. Open decisions

- G-MCP1. Resolved 2026-10-28: see `mcp-registration.md`. Claude Code user scope through `claude mcp add -s user`; Codex
  through the `[mcp_servers.downshift]` block; tool names are plain verbs under the server name `downshift`.
- G-MCP2. Whether `delegate` returns after the job starts or waits (default: returns a job id; `result` blocks with a
  timeout). Owner to confirm.

## 4. Where this differs from the business plan repository

`downshift-business-plan` still describes the earlier design: Downshift picks `selected_model` from `kind` and
`effort_class` and applies a risk floor (section 1 of the orchestration spec, and the `delegate` return value).
Owner decision O8 (2026-10-28) replaces that: the orchestrator passes `model` and `effort`, Downshift applies and
reports them, and there is no built-in floor. The business plan repository should be updated to match; this file
does not edit it. The playbook editor (`downshift playbook`) and the `playbook` parameter are not in that repository
at all.

## 3. Protocol coverage

Read from the specification (revision 2026-07-28, 2026-10-28). Every implementation must support the base protocol,
versioning and the message patterns; the rest is optional.

| Part of the protocol | Required | In our server |
|---|---|---|
| Base protocol, JSON-RPC 2.0, error codes | Yes | Yes (the SDK) |
| Versioning: per-request `_meta` version, `server/discover`, `UnsupportedProtocolVersion` (-32022) | Yes (modern era) | Yes (the SDK; tested) |
| Legacy handshake (`initialize`, `notifications/initialized`) | For clients of 2025-11-25 and earlier | Yes (the SDK; tested) |
| Tools: `tools/list`, `tools/call` | Optional feature | **Yes: the whole point** |
| Cancellation, ping, progress | Utilities | Cancellation and ping through the SDK; progress when `delegate` needs it |
| Resources, prompts | Optional features | No. Playbooks could be offered as resources later |
| Logging, completion, subscriptions, multi round-trip requests | Optional | No |
| Client features: sampling, roots, elicitation | Optional (client side) | No |
| Extension Tasks (async long-running calls with polling) | Opt-in extension | To evaluate for `delegate`, instead of our own `seq`/`ack` of R14 |
| Extension Skills over MCP | Opt-in extension | To evaluate for playbooks |
| Streamable HTTP and OAuth | Other transport | No (O7: stdio only) |

Behaviour of the library that the tests pin: a line that is not JSON ends the session, and so does a message above
the 1 MiB cap; the protocol is stateless, so the client starts a new process. When the input closes while several
requests are in flight, the answers still pending are lost (observed with 20 requests sent at once); a client that
waits for each answer is not affected.

## 5. Lessons for the worker tools (clean-room behaviour analysis, 2026-10-28)

Source: a behavioural analysis of a third-party orchestrator's MCP path, written by one reader and kept in the
private `downshift-labs` repository. It holds no code, prompt text or identifiers, and the implementer never reads
the source it describes. The points below are requirements of ours that the analysis made obvious. They apply when
`delegate` and its siblings are built (stage 4, M2b); none of them changes the information tools.

- R18. Where the timing guidance lives. A tool description is read only after the model has chosen a tool, so the
  rule for *when* to call goes in the server instructions (done: short, first sentence) and in what each result says
  next (done for `providers`: the `next` field). Both are the only channels that reach the orchestrator mid-session.
- R19. Deliveries and acknowledgements are keyed by the calling session, never global, so two orchestrators on one
  server cannot swallow each other's results.
- R20. The delivery sequence carries a per-boot epoch (or is persisted), so an acknowledgement held across a restart
  cannot drop new results.
- R21. A delivery has a size cap and a way to page; the status view trims every field. A result above the client's
  tool-result limit fails the whole call, and the information tools already cap at 64 KiB.
- R22. `result` waits at most 45 seconds and is called again; a client drops a call that blocks much longer. The
  tools must not serialize behind a waiting call: `cancel` has to work while `result` waits.
- R23. A worker waiting on a person is reported as its own state in `result`, not only in `status`, and releases its
  concurrency slot accounting exactly once on every ending (finish, cancel, release, timeout, lost output).
- R24. A worker with no output for a bounded time is stopped (idle limit), and a budget of zero is not accepted,
  so a hung process cannot hold a slot forever. The wall-time budget is per turn, not per process.
- R25. The tail of a worker's standard error (bounded) goes into the failure result; "spawn failed" alone is not
  enough to act on.
- R26. A worker must not be able to call Downshift's own delegation tools (recursion). The worker contract and the
  environment both say so, and the information tools stay available.
- R27. Read-only is never silently dropped: a mode a provider cannot honour is refused with the reason, as R15
  already requires. Which Claude permission mode really blocks writes with the long-lived stream is unverified
  and must be tested before `mode: read` is offered for Claude.
- R28. Quota readings come from what the provider's own CLI reports or from snapshots the user imports (the
  decision already taken); stored login tokens of other tools are not read. A reading that stops refreshing expires
  (R13 already treats a missing reading as "let work through").
- R29. Results of interrupted or lost workers can be recovered: the session id is stored, the result text is not.
- R30. A worktree is created per write worker and is never removed implicitly; the cleanup command lists and
  removes only the ones Downshift made, by marker. Uncommitted changes of the main checkout do not reach the
  worktree, and the result says so.

What the analysis says a lean server can skip, and ours does: terminals, a rules engine with quota gates and role
fallback chains, an approval round trip, supersession, run labels, a second runner with more providers, and
reading other tools' credentials.

## 6. R27 status (2026-10-29)

- Claude: the read-only launch is print mode with the planning permission mode and the prompt on standard input.
  The recipe follows the public behaviour of another orchestrator's secondary route; its own authors did not
  test that the planning mode blocks writes. The vendor docs say permission rules are enforced by Claude Code, not
  by the model, and that a write in plan mode goes through the normal permission flow. Not observed: in three
  real runs the model chose not to attempt a write, so no denial was seen.
- Codex: exec with the read-only sandbox, which the Codex help lists as a sandbox mode. Not observed on a real run.
- Both launches are off by default. `DOWNSHIFT_WORKERS=read` turns them on. The first real run decides R27.

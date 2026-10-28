<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# MCP delegation server (`downshift mcp serve`)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Status: **Draft, 2026-10-28.** Reopens owner decision O7 for one part: the MCP server is Downshift's own, run by the
same binary, not a separate product. Names follow the business plan repository (`architecture/multi-vendor-orchestration-spec.md`, section 2): `downshift mcp serve` and tools named `downshift.<verb>`. Decision O8 (the LLM chooses the model) is the premise. Clean-room: the behaviour
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
- R3. Tools: `downshift.delegate` (harness, model, effort, mode `read|write`, prompt, cwd, optional `playbook`,
  optional `verify_argv`, optional `slug`; `kind` and `effort_class` are hints only), `downshift.status` (queue,
  phase per task, strain per quota window), `downshift.steer` (interrupt or steer a running task),
  `downshift.result` and `downshift.cancel` (task id), `downshift.providers` (installed agent CLIs and their models
  from the catalog), `downshift.playbook` (no name: the valid names; a name: that playbook's text).
- R10. `downshift.delegate` with a `playbook` name prefixes the worker's first message with
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
- R13. Every `downshift.*` result carries a `strain` block: the fullest window per provider and its headroom name
  (`ok`, `strained`, `critical`, or unknown). It is advice for the orchestrator, never a reroute, and it is
  computed at call time. A host cannot push to the model, so the reading rides on the answer it already reads.
  (Survey idea A1.)
- R14. `downshift.result` is repeatable and acknowledged. Each delivery has a sequence number; the caller acks it
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

The MCP is part of the MVP (owner decision O9, 2026-10-28), sliced: M1, M2a, M3 and M4 are stage 2b; M2b comes with the plan runner (stage 4).

| Front | Points | Scope | Depends on | Gate |
|---|--:|---|---|---|
| M1 JSON-RPC stdio core (MVP, K3) | 3 | `internal/mcp/rpc/*`: framing, initialize, tools/list, tools/call, errors (R1, R9) | none | unit tests on framed messages; `make check` |
| M2a Information tools (MVP, K4) | 3 | `internal/mcp/tools/*`: `downshift.providers` (detected models), `downshift.playbook`, `downshift.status`, `downshift.apply`; the strain block (R13) on every result | M1, K1, K2 | tool tests; no prompt on disk |
| M2b Worker tools (stage 4) | 3 | `delegate`, `steer`, `result`, `cancel` with R10, R14 to R17, over the plan runner's jobs | M2a, P8 | R3, R6, R7, R14 tests |
| M3 Command (MVP, K5) | 1 | `cmd/downshift/cmd_mcp.go`, `case "mcp"` in `main.go` (`serve`), help spec | M2 | e2e: initialize + tools/list over pipes |
| M4 Setup (MVP, K6) | 2 | `setup` registers the server in the harness's MCP config (opt-in, dry-run, undo by marker) | M3, S1 | setup tests; undo leaves other servers |
| M5 Playbook | 1 | `downshift-orchestrator` skill names the MCP tools (after M2) | M2, R2 | `TestSetupInstallsTheOrchestratorPlaybook` |

Dependency direction: `internal/mcp` depends on `internal/plan` through small interfaces; nothing in `internal/routing`
or `internal/core` imports `internal/mcp`. Each front is one PR of at most 5 points, `make check` green.

## 3. Open decisions

- G-MCP1. Harness registration path for Claude Code (`.mcp.json` or the settings `mcpServers` key). Verify on the
  installed version before M4.
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
the 1 MiB cap; the protocol is stateless, so the client starts a new process.

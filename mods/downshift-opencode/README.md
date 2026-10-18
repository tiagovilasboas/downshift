# downshift-opencode (experimental mod)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

An opt-in **OpenCode v2 mod** (project-scoped plugin) that forwards a subagent spawn to Downshift's
`opencode` hook (wire v1), applies a `route` answer with `ctx.session.switchModel`, and sends the
post-spawn observations back to `downshift honor opencode` and `downshift end opencode`. All
classification, policy, guardrails and telemetry stay in the Go core; this file is only the caller
side of the contract documented in
[internal/adapters/opencode/opencode.go](../../internal/adapters/opencode/opencode.go).

**Status: experimental (Beta harness integration).** Marker-gated, fails open, never denies, and it
never restarts or blocks the harness (see [Fail-open](#fail-open-n1)).

## What it does

While the project marker exists, on each `subagent` spawn:

1. `execute.before` records the spawn intent (agent name, task text in memory only) for the parent
   session.
2. On the child's first `session.prompt` the plugin runs `[bin] opencode` — one route request on
   stdin, one strict answer on stdout (2 s deadline, stdout capped at 1 MiB) — and, when the answer
   is `route` to a different model, calls `ctx.session.switchModel` for the child before its first
   `context` hook. The switch, the bus `session.model.selected` and the child's first
   `session.context` are recorded as observation envelopes.
3. On `session.tool.success` of the tracked parent, the arrival-ordered batch
   (`switch_attempt` → `model_selected` → `child_context` → `child_done`) is sent to
   `downshift honor opencode` and `downshift end opencode`. One flush per child.

Without the marker the plugin is dormant: zero binary calls, zero state (the child prompt logs one
content-free `gated` line).

## Install it (manual)

OpenCode v2 discovers plugins per project directory; there is no installer yet (that is phase 3).

```bash
PROJ=/path/to/project
mkdir -p "$PROJ/.opencode/plugins/downshift-opencode"
cp index.ts "$PROJ/.opencode/plugins/downshift-opencode/"
# config.json lives next to index.ts, mode 0600, and is never committed:
#   {"bin":"/abs/path/downshift","binSHA256":"<sha256 of bin>","salt":"<32 random bytes, 64 hex chars>",
#    "logFile":"/abs/path/plugin.jsonl","env":{"DOWNSHIFT_STATE_DIR":"/abs/state","DOWNSHIFT_SESSION_MODELS":"/abs/session-models.json"}}
chmod 600 "$PROJ/.opencode/plugins/downshift-opencode/config.json"
# enable for this project:
touch "$PROJ/.dsspike-switch-enabled"
# re-instantiate the plugin (never restart the service):
opencode reload
```

Generate the salt once per install (`openssl rand -hex 32`) and keep it private: it is the HMAC key
that pseudonymises every session id in the log. The setup is fail-closed — an absolute, owned,
non-writable bin path, a matching sha256 pin, a valid salt and a 0600 log file are all required
before any hook is armed, and every failure is one content-free `plugin.setup` line.

## Wire

- `[bin] opencode` — route request (stdin) → one strict `{"v":1,"action":"allow"|"route",...}` line
  (stdout). Response keys are an allow-list (`v,action,model,reasonCode,ttlMs`); anything else is
  treated as `allow` with `reasonCode:"fail_open"`.
- `[bin] honor opencode` and `[bin] end opencode` — the observation batch (NDJSON, arrival order,
  one child per batch). Envelope keys: `v,kind,correlation{child,parent},event,from,to,model,
  previous,result,api,status,seq,tsUtc`; lines ≤8 KiB; no `usage` (see [Limits](#limits)).

## Privacy

- `taskText` is a transient classification input: memory → the local binary's stdin. It is never
  logged, never persisted and never echoed back.
- The plugin log (`logFile`, 0600) carries allow-listed keys only: timestamps, HMACs, model ids,
  codes, timings and exit codes. Zero session ids (`ses_…`), zero paths, zero prompt/tool text, zero
  console mirror.
- No network access of any kind: the only imports are `node:fs`, `node:crypto`,
  `node:child_process`, `node:path` and `node:url`; the binary is spawned by absolute, sha256-pinned
  path with an argv array; the child env is `{HOME, TMPDIR, DOWNSHIFT_*}` only.
- The bus guard processes only allow-listed event types whose session is tracked or whose location
  resolves (realpath) to this project; payloads are projected, never forwarded raw.

## Personal/public use only (ZDR)

The routed `*-free` catalog models have **no verifiable zero-data-retention contract**. Use this mod
only for personal or public work — never for corporate code, secrets or personal data. The real data
gate (class-of-data enforcement at install time) is phase 3.

## Fail-open (N1)

Binary missing at runtime, non-zero exit, timeout, oversized output, malformed answer, or a failed
switch: the child simply keeps its original model and one `route_fail_open` (or `switch_attempt
result=error`) line records the code. A failed switch never produces an observation envelope, so the
honor side can never claim a rewrite that did not land.

## Limits

- One tracked child per parent per spawn; the flush is triggered by the parent's
  `session.tool.success` — no bus event, no flush (observations are then lost; documented).
- `usage` is **NOT PROVEN**: `child_done` carries no tokens and nothing is priced.
- OpenCode v2 only (plugin API generation); project-scoped (one project dir, one marker).
- The plugin answers the harness side of a rewrite; it never claims the provider served the model.

## Uninstall / rollback

```bash
PROJ=/path/to/project
rm -f "$PROJ/.dsspike-switch-enabled"                 # dormant first
rm -rf "$PROJ/.opencode/plugins/downshift-opencode"   # remove the plugin
opencode reload                                        # re-instantiate
# delete the log file and the salt when the telemetry is no longer needed
```

`reload` is the only lifecycle operation used; the OpenCode service is never restarted.

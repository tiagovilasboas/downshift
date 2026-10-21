# OpenCode spike: how the requested model reaches the decision (T2, T3)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Date: 2026-10-21. OpenCode 2.0.32 (`/opt/homebrew/bin/opencode --version`). No paid run.

## Finding

The "requested model unknown, held" line was **not an OpenCode gap**. It came
from the `downshift try` command line. `try` has no `--harness` flag: the
usage is `try "<prompt>" [harness] [current-model]`. In
`downshift try --harness opencode "rename the userId variable ..."` the first
argument (`--harness`) became the prompt, `opencode` the harness and the task
sentence the current model. A made-up current model is not in the catalog, so
the verdict was UNKNOWN, and a prompt with no signal falls to mid and is held
(R9). Reproduced on 2026-10-21 with `DOWNSHIFT_STATE_DIR` on an empty dir:
`→ mid: glm-5.3-flash-free ... the requested model was unknown and the read was unsure`.

## Where OpenCode carries the requested model

| Fact | Status | Source | Date |
|---|---|---|---|
| A subagent's model is the `model` key of its agent entry (`opencode.json` `agent.<name>.model`, or agent frontmatter), form `provider/model-id`; unset means the invoking primary agent's model, and a primary without one uses the global model | CONFIRMED | https://opencode.ai/docs/agents/ | 2026-10-21 |
| A child (subagent) session stores its resolved model as an object `{"id","providerID","variant"}` and its `parent_id`, plus `agent` | CONFIRMED on this machine: `session_v2.model` / `parent_id` / `agent` in `~/.local/share/opencode/opencode.db`, 11 child rows, for example `{"id":"big-pickle","providerID":"opencode","variant":"default"}` with agent `explore`, version 2.0.32 | local DB, schema read with `sqlite3 .schema`; no prompt content read | 2026-10-21 |
| The repo plugin reads that object (`ctx.session.get(...).model`) when the child's first prompt arrives and sends it as `model` in the wire v1 request | CONFIRMED in code | `mods/downshift-opencode/index.ts` (session.prompt hook), `internal/adapters/opencode/opencode.go` (`RequestedModel()`) | 2026-10-21 |
| The adapter hands `model.id` to `core.Route` as the current model and to `Baseline.RequestedID`; core decides as for every harness | CONFIRMED in code and test | `opencode.Handle`, `TestHandle_ChildSessionModelObjectIsTheRequestedModel` | 2026-10-21 |
| That `ctx.session.get` already holds the resolved model at the moment of the `session.prompt` hook of a brand-new child | UNKNOWN: needs one live spawn. If it is empty the plugin fails open with code `no_model` (a different, visible reason), never "unknown" | `index.ts` `failPre("no_model")` | 2026-10-21 |
| Plugin API page naming the session object fields | UNKNOWN: not re-verified here; the plugin's use of `ctx.session.get` was built against 2.0.32 | https://opencode.ai/docs/plugins/ | not fetched |

`opencode --help` and `opencode agent --help` expose no model field for a
spawn; the DB row above is the only local evidence of the shape.

## Smallest fix (T3, done)

No plugin or adapter change is needed: the field exists and is parsed into the
same current-model slot as every other adapter. The defect was the `try`
argument handling, fixed in the shared CLI layer:

- `downshift try` accepts `--harness H` and `--model M` (or `--flag=value`),
  anywhere in the arguments; the positional form keeps working.
- A leading unknown `--flag` is an error, never the task prompt.

Tests: `TestStripTryOptions` (cmd/downshift) and
`TestHandle_ChildSessionModelObjectIsTheRequestedModel` (opencode adapter wire
shape with `variant`).

## Live check

State dir with `openrouter.json` (`deepseek/deepseek-v4.1-flash-1017`, tier
small, route live) and `session-models.json`
(`{"opencode":["deepseek-v4.1-flash-free","big-pickle"]}`):

```
downshift try --harness opencode "rename the userId variable to userIdentifier"
→ small: big-pickle → deepseek/deepseek-v4.1-flash-1017 · its tokens would run on a lower tier than requested
```

With `--model glm-5.3-flash-free --detail`: `Verdict: DOWNSHIFT`,
`Rewrite: yes → deepseek/deepseek-v4.1-flash-1017`.

## Open item

One live OpenCode spawn with the plugin marker enabled would settle the single
UNKNOWN above (child model present at `session.prompt`). Watch the plugin log
for `no_model`.

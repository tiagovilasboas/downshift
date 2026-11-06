<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# `downshift probe`: what does a hook payload really carry?

A maintainer tool for questions the vendor documentation cannot settle. Cursor's docs, for
example, say `preToolUse` carries `model`, `model_id` and `transcript_path`, but the first
144 captured Task payloads had an empty `model`. The probe records what arrives.

Wired as a hook, `downshift probe <event>` appends one line to `<state dir>/probe.jsonl`
and answers the harness so nothing is blocked (`{"permission":"allow"}` for `preToolUse`,
`{}` for every other event, exit 0 always).

## What it records, and what it never does

Recorded:
- the event name and a timestamp;
- the names of the payload's top-level fields and of `tool_input`'s fields;
- the value of a short list of fields that are not sensitive: `model`, `model_id`,
  `subagent_model`, `tool_name`, `subagent_type`, `composer_mode`, `source`,
  `hook_event_name`, `cursor_version` (and the same under `tool_input.`). An empty `model` is
  recorded as empty, which is the thing to learn;
- an 8-character hash of each id (`conversation_id`, `generation_id`, `session_id`,
  `tool_call_id`, `subagent_id`, ...), so two events can be joined without exposing the id;
- `present` for `transcript_path` and `agent_transcript_path`, never the path.

Never recorded: prompts, task text, file paths, workspace roots, e-mail addresses, or an id
in the clear. Recording stops at 2 MiB.

## Cursor: does the session model reach a hook?

Question: for a Task whose `model` is `inherit` or absent, can the baseline be resolved from
Cursor's own payloads? Wire the probe next to the existing hooks in `~/.cursor/hooks.json`,
using the absolute path of the binary:

```json
{
  "version": 1,
  "hooks": {
    "sessionStart":       [ { "command": "/path/to/downshift probe sessionStart" } ],
    "beforeSubmitPrompt": [ { "command": "/path/to/downshift probe beforeSubmitPrompt" } ],
    "preToolUse":         [ { "command": "/path/to/downshift probe preToolUse" } ],
    "subagentStart":      [ { "command": "/path/to/downshift probe subagentStart" } ],
    "postToolUse":        [ { "command": "/path/to/downshift probe postToolUse" } ]
  }
}
```

Keep the existing entries; add these to each event's list. Run one conversation in Cursor that
spawns a Task (one with an explicit model, one without), then read the file:

```bash
python3 - <<'PY'
import json, os
for l in open(os.path.expanduser("~/.downshift/probe.jsonl")):
    r = json.loads(l)
    print(r["event"], r.get("fields", {}), r.get("top_keys"))
PY
```

What to look for:
1. Does `sessionStart` or `beforeSubmitPrompt` carry `model` or `model_id`, and is it non-empty?
2. Does `preToolUse` for the Task carry a non-empty `model`, `model_id` or `transcript_path`?
3. Does a `conversation_id` hash on an earlier event equal the one on the Task's `preToolUse`?
   If so, the session model could be remembered per conversation and used as the baseline.
4. In what order do `preToolUse` and `subagentStart` arrive, and does `subagentStart`'s
   `subagent_model` match what the Task asked for?

Remove the probe entries from `hooks.json` when done; `downshift setup --undo` restores the
last backup if setup was the last thing to write the file.

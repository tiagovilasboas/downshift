# agent-desktop: compact desktop reads

Downshift by Tiago Vilas Boas
https://github.com/tiagovilasboas/downshift-ai

`agent-desktop` is a command-line tool that lets a coding agent drive desktop apps on macOS, Windows and
Linux. It reads the app's accessibility tree instead of taking screenshots, prints it as JSON with a ref
(`@s8f3k2p9:e12`) for each element, and acts on refs. The agent reads the UI again after every action to check
what happened, so the same tree comes back many times.

Downshift does not install `agent-desktop` and does not drive the desktop. It shortens what `agent-desktop`
prints, in two steps:

1. **Compact view.** A `snapshot` or `find` envelope is re-encoded one line per element. Every field is kept.
2. **Delta.** A repeat read of the same window in the same session is sent as the lines that changed.

## Numbers

Measured with the `downshift` binary on synthetic fixtures built to `agent-desktop`'s output schema
(`internal/compressor/testdata/corpus/agent_desktop_*`), not on a live desktop. Measure on your own desktop
before quoting them.

| Read | Raw JSON | Printed | Saving |
| --- | --: | --: | --: |
| File manager window, first read (219 refs) | 30,272 B | 14,183 B | 53.1% |
| Chat app window, first read (175 refs) | 26,861 B | 10,802 B | 59.8% |
| `find`, 18 buttons | 3,899 B | 1,455 B | 62.7% |
| Same chat window after an action enabled Send (2 lines changed) | 26,839 B | 764 B | 97.2% |
| Same chat window, nothing changed | 26,839 B | 567 B | 97.9% |

Printed bytes include the marker line. Over the three chat reads (read, act, verify twice), 80,539 bytes
become 12,133 bytes: 84.9% fewer. `agent-desktop`'s own `-i`, `--compact` and `--skeleton` flags still apply
first and stack with this.

Each read is a normal Compactor run: it is logged in `exec-events.jsonl` under the family `agent-desktop` and
counted in `downshift report`, `downshift stats`, the session panel and the `downshift-agents` pane. Bytes read
back with `downshift retrieve` are subtracted, as for every family.

## How a read gets compacted

| Where | What to do |
| --- | --- |
| Claude Code with the Downshift hook | Nothing. `agent-desktop snapshot ...` and `agent-desktop find ...` are rewritten to `downshift compact` with the session. |
| Any other harness, or a script | Run `downshift compact -- agent-desktop snapshot --app Files -i`. Both steps apply; the delta is scoped to the working directory instead of a session. |
| `agent-desktop mcp` (MCP tools) | Not compacted: Downshift does not see MCP tool results. Use the CLI form to get the view and the delta. |

Only `snapshot` and `find` are wrapped, with the subcommand first. A global flag before it
(`agent-desktop --session s snapshot`), `--debug`, `--screenshot`, `--help` and `--version` run as typed.
Actions (`click`, `type`, `set-value`, ...) are never wrapped. A failed read (`"ok": false`, exit 1) prints
byte for byte.

## The compact view

```text
agent-desktop snapshot (envelope 2.6) app="Files" complete=true ref_count=219 snapshot_id="s3kq9x2m7a" window={"id":"w-31337","title":"Downloads"}
refs: eN means @s3kq9x2m7a:eN
actions: an eN line without actions= has Click; actions=- means none
window "Downloads"
  toolbar
    e1 button "Back" id=ax_identifier:"back"
    e2 button "Forward" [disabled]
    e5 searchfield "Search" actions=Click,SetFocus ph="Search"
```

- The first line holds every `data` field except the tree, as JSON.
- Two spaces of indent per level stand for the nested `children`.
- A ref of this snapshot is written `eN`. Use it as `@<snapshot_id>:eN` from the legend.
- `[a,b]` are states, `box=x,y,w,h` are bounds (with `--include-bounds`), `id=kind:"value"` is the native id,
  `.a.b` are DOM classes, `more=N` counts children not read, and `cut` marks a truncated subtree.
- `value=`, `desc=`, `ph=` (placeholder), `rdesc=`, `sub=` and `hint=` are JSON.
- Any field without a short form is written `key=<json>`, so a field agent-desktop adds later is kept too.
- A `find` view leaves out `value=null`, empty states and `interactive=true`, and writes `path=^` when the path
  is the one on the line above.

A test decodes the view back and compares it with the input JSON (`internal/compressor/agent_desktop_roundtrip_test.go`).

## The delta

agent-desktop numbers refs depth-first over the elements it can act on. When the tree has not changed, the
numbers are the same and only the snapshot id is new. Refs of an older snapshot stay valid: agent-desktop keeps
up to 128 snapshots. So a line that reads the same in both views is the same element with the same ref.

```text
delta from snapshot s9x0lmz4 (same window, read 4s ago): 4 of 212 lines changed. Every other line of that view still holds, and its eN refs are valid in both snapshots. If you do not have snapshot s9x0lmz4, read the full output below.
@@ line 210
     e173 button "More actions" .btn.msg-action
-    e175 button "Send now" [disabled]
+    e175 button "Send now"
[downshift compact: compacted 30899 to 868 bytes; full output: downshift retrieve 0123456789ab]
```

A delta is sent only when all of these hold. Otherwise the whole view is printed:

- The command line is exactly the same as the previous read, in the same scope, within 10 minutes. The scope
  is the hook's session, or the working directory for a direct `downshift compact` call.
- Both reads are complete. A read with `complete=false`, `truncated=true` or `identity_unproven=true` never
  gets a delta, because its missing nodes are not removals.
- The legends match.
- The raw output was kept by [CCR](ccr.md), so `downshift retrieve <id>` can always give the full read.
- The delta is under 60% of the view.

A new element early in the tree renumbers every ref after it. The delta then grows past 60%, and the full
view is sent.

The previous view is kept under `~/.downshift/desktopview/`, one private file (0600) per window and scope.
Lines that look like secrets are withheld, the same way CCR withholds them. The file name is a hash, not the
app or window name. Set `DOWNSHIFT_DESKTOP_DELTA=0` to send every read whole. The compact view stays on with
the compactor (`DOWNSHIFT_EXEC=0` turns both off).

## Limits

- **Shared scope.** Subagents in one Claude Code session share its session, and direct calls from one
  directory share that directory. An agent that reads a window another agent read a moment before gets a
  delta against a view it never saw. The delta says so and names `downshift retrieve`, but that costs a round
  trip. Set `DOWNSHIFT_DESKTOP_DELTA=0` if several agents read the same windows.
- **Secrets.** agent-desktop prints the whole envelope on one line. If any field looks like a secret, CCR
  withholds that one line, which is the whole raw read. `retrieve` then returns no tree. The compact view and
  the delta are unaffected.
- **Measurement.** Every number above comes from synthetic fixtures. A live desktop measurement is still to
  be done: run the same task with `DOWNSHIFT_EXEC=0` and with the hook on, and compare `downshift report`.

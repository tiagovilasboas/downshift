# Claude Code — Downshift v0.4.0 real-session evidence (2026-11-01)

This records a real Claude Code session that exercised Downshift v0.4.0 end to
end: the MCP server (all 11 tools), the compactor, and subagent routing. Every
result below is copied from live output captured during the run.

- Binary: `downshift 0.4.0 (commit 3d9c382c)`, on `PATH` via `~/.local/bin`.
- Harness: Claude Code `2.1.316`, plugin `downshift@downshift` enabled, 8 hooks
  wired, MCP `downshift` registered and answering 11 tools over stdio.
- Scratch: a throwaway git repo (`git init`, a Go file, one commit). Write
  workers ran with `DOWNSHIFT_WORKERS=bypass` set **only** in the test
  environment and **only** in the throwaway repo, never in `$HOME` or a real
  project.
- `doctor` before the run reported, for `claude-code`: plugin enabled, hooks
  wired, `mcp registered`, `mcp answers 11 tools listed over stdio`,
  `rewrite honored seen: 13 of 15`.

## 1. MCP — the 11 tools

The four information tools were driven by asking the installed `claude -p`
(headless, `--output-format json`) to call them; each returned a real result
with `is_error: false` and no permission denials. The seven worker tools were
driven over `downshift mcp serve` by stdio (the server the plugin registers),
with a bypass worker enabled for this repo only, because the plugin's own
server has workers off by design (`delegate refuses every task` otherwise). The
task asks used trivial, cheap prompts tagged `[tier=small]`.

| Tool | Proved | How |
|------|--------|-----|
| `providers` | yes | real Claude session called `mcp__downshift__providers`; returned the model list with tiers and a strain block |
| `apply` | yes | real Claude session; verdict `kept`, `reason: requested model kept: no declared policy or hint moved it` |
| `playbook` | yes | real Claude session; listed 9 playbooks with sizes |
| `status` | yes | real Claude session; `server downshift, version 0.4.0, playbooks 9` |
| `delegate` (read) | yes | stdio; worker returned its one-word answer with real token usage and a session id |
| `delegate` (write) | yes | stdio; worker ran in its own git worktree on branch `downshift/w002` |
| `collect` | yes | stdio; delivered both worker results; a second call flagged them `repeat: true` after `ack` |
| `diff` | yes | stdio; returned the unified diff the write worker produced (new file `NOTE.txt`) |
| `send` | yes (live turn) | stdio; accepted a follow-up on a still-running read worker, queued at `position 1` on the same conversation |
| `steer` | yes (live turn) | stdio; caught a live turn (`state: steering`, `position 0`); the collected result said the turn was interrupted so the correction could run first |
| `cancel` | yes | stdio; `{"cancelled":["w001"]}`, the worker's collect outcome became `cancelled` |
| `release` | yes | stdio; `{"released":["w001"]}` and the worker's transcript was deleted (see §2) |

Selected raw output:

```
# apply (real Claude session)
{"data":{"harness":"claude-code","requested_model":"...","selected_model":"...",
"tier":"small","verdict":"kept","reason":"requested model kept: no declared
policy or hint moved it","applied":false,...},"strain":[...]}

# delegate write result (stdio)
"done\n\nworktree: .../proof/.downshift/worktrees/w002 (branch downshift/w002).
Review the diff with git before you merge; Downshift does not merge."

# diff of the write worker (stdio)
diff --git a/NOTE.txt b/NOTE.txt
new file mode 100644
--- /dev/null
+++ b/NOTE.txt
@@ -0,0 +1 @@
+proof

# send on a still-running worker (stdio)
{"data":{"job":"w001","position":1,"state":"queued"},...}

# steer catching a live turn (stdio)
{"data":{"job":"w002","position":0,"state":"steering"},...}
# then, collected:
"this turn was interrupted by steer so a correction could run first; work the
turn had not yet saved to the session is lost, but anything it wrote to the
worktree remains"
```

### Finding: `send` / `steer` need a resumable session

`send` and `steer` succeed while a worker is on a live turn (proved above). The
earlier note recorded `send` on a **finished** read worker refusing with `that
worker cannot take another turn: it was cancelled, failed, or captured no
session to resume`, and read it as the documented guard rather than a defect.
That reading was wrong: the finished worker **had** captured a session id, so it
should have resumed. A live reproduction found and fixed a real defect.

**Defect.** `canResume` (`internal/workers/continuation.go`) only allows a
resume when the queue's launcher implements `resumeLauncher` (`LaunchResume`).
Every real MCP server builds its queue through `Routes(read, write)`
(`cmd/downshift/cmd_mcp.go` `workerLaunchers`), and `routes`
(`internal/workers/write.go`) implemented `Launch` and `CleanupSession` but
**not** `LaunchResume`. So `q.launcher.(resumeLauncher)` was always false and a
settled `done` worker was refused a second turn even though its inner launcher
(`byAgent` → `cliLauncher`) could resume it. The package's continuation unit
tests passed a resume-capable launcher directly, bypassing `routes`, which is
why the gap was invisible until a live run through the server.

**Live reproduction, before the fix** (`downshift mcp serve`, stdio,
`DOWNSHIFT_WORKERS=bypass`, disposable git repo). A `claude` read worker,
task "reply with the single word: pineapple `[tier=small]`", collected to
`done` with a real session id:

```json
{"seq":...,"job":"w001","outcome":"done","text":"pineapple","session_id":"9f692fcb-0fea-49a0-a194-502139fe1c93"}
```

`status` still showed that `session_id`, yet `send` with a follow-up
("repeat the exact word you just said") was refused:

```
TOOL_ERROR that worker cannot take another turn: it was cancelled, failed, or captured no session to resume; start a new worker with delegate
```

**Fix.** `routes` now forwards `LaunchResume` to the launcher of the worker's
mode (read or write), exactly as it forwards `CleanupSession`. A nil write
launcher is refused as `ErrWriteOff`; a route whose launcher cannot resume is
refused the same way its launch is.

**Live reproduction, after the fix** (same steps, fresh repo, binary built from
the fix branch). First turn `done`, session `ffa6a2cb-787c-435e-a255-29941423d668`.
`send` was accepted:

```json
{"job":"w001","state":"resumed","position":0}
```

and the resumed turn's delivery, on the same session id, answered the follow-up
with the word from the first turn:

```json
{"seq":...,"job":"w001","outcome":"done","text":"pineapple","session_id":"ffa6a2cb-787c-435e-a255-29941423d668"}
```

So the second turn remembered the first: the resume path works end to end.

**Regression test.** `TestRoutesForwardsResumeSoASettledWorkerResumes`
(`internal/workers/resume_test.go`) builds the queue through `Routes` like the
real server and proves, unhappy-path first (1:3): a settled worker with no
captured id is refused, a cancelled worker is refused, and a settled `done`
worker with an id resumes with `--resume <id>` in the same dir. It fails before
the fix and passes after.

## 2. Transcripts deleted on release

Claude Code stores a worker transcript at
`~/.claude/projects/<cwd-encoded>/<session-id>.jsonl`, where the folder is the
worker's working directory with `/` and `.` replaced by `-`. The worker id is
pre-set by Downshift on `--session-id`
(`internal/workers/launch_agents.go`), and `release` runs the cleanup that
deletes `<folder>/<session-id>.jsonl` plus the `session-env/<session-id>` dir
(`internal/workers/cleanup.go`).

- Read worker, session id `625ec4d7-…`: after `release`, a search of
  `~/.claude/projects` for `625ec4d7-….jsonl` returned nothing, while an
  unrelated session transcript in the same folder was still present. So release
  deleted exactly the worker's transcript, not the folder.
- Worker cancelled on its first turn, session id `4762d15d-…` (pre-set via
  `--session-id`, confirmed `running` by `status` before cancel): `cancel`
  returned `{"cancelled":["w001"]}`, collect outcome `cancelled`, `release`
  returned `{"released":["w001"]}`, and the `4762d15d-….jsonl` transcript was
  gone from disk.

## 3. Compactor

Run inside the throwaway repo; byte deltas read from the exec-events log.

```
downshift compactor -- go test ./...      # tiny cached output, not compacted by design
downshift compactor -- git status         # 186 bytes, below threshold, not compacted
downshift compactor -- go test -v -count=1 ./...   # 9 passing tests, verbose
  stdout: Go test -v: PASS (9 passed, 0 failed, 0 skipped) in 0.665s
          [downshift compact: compacted 403 to 74 bytes]
  exec event: family=go-test raw_bytes=403 emitted_bytes=121 compacted=true  (≈70% fewer bytes)
```

The verbose all-pass run was collapsed to a one-line summary. The two tiny
outputs were correctly classified by family and left untouched because they are
below the compaction threshold — the compactor did not inflate them.

## 4. Routing

A real Claude Code subagent spawn (the Task tool), events read from the routing
log. Two spawns: one hinted `[tier=small]`, one with no hint.

```
# [tier=small] spawn — hint_down, honored
decision:    verdict=DOWNSHIFT  tier=small  outcome=rewrite_emitted
             requested_model=<mid>  final_model=<small>
             routing_note={verdict: hint_down, tier_from: mid, tier_to: small}
             reason_codes=[CLASSIFIER_CONFIDENT]  correlation_id=24207c80…
honored:     rewrite_honored=true  linked_decision=24207c80…
child_ended: final_model=<small>  linked_decision=24207c80…

# no-hint spawn — kept
decision:    verdict=OK  outcome=allow  tier=mid
             requested_model=<mid>  final_model=<mid>
             routing_note={verdict: kept, tier_from: mid, tier_to: mid}
child_ended: final_model=<mid>
```

The hint moved the tier down (mid → small) and the child ran the rewritten
model; the hint was recorded as honored and the `child_ended` record confirmed
the model that actually ran. Without a hint the model was kept and the child ran
the requested model. (Model names appear here only as observed facts in the
logs; they are not a recommendation — choose by tier.)

## Registry update

`internal/limits/registry.go` previously said 7 of 11 MCP tools were answered in
a real session and four (diff, send, steer, release) were unit-tested only. This
session exercised all 11, so the `mcp` limitation for `claude-code` now points
here and states all 11 were exercised in a real session. The `internal/limits`
and `cmd/downshift` test suites pass with the change.

## Cleanup

All workers were released; the throwaway repo was removed with `rm` of known
files and `rmdir` (no `rm -rf`); no worker session for the proof folder remained
under `~/.claude/projects`.

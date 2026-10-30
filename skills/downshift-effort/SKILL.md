---
name: downshift-effort
description: Advice for the orchestrator on matching reasoning and verification effort to what a task puts at risk. Use when deciding how hard to think, how much to check, or what effort to give a subagent, with Downshift installed.
---

# Effort follows the stakes

Spend care where a mistake is costly, and nowhere else. This skill is advice to you, the
orchestrator. It never sets an effort value: you choose the model and the effort of every
spawn, and Downshift only applies your choice within the project policy (owner decision O8).

## 1. Read the stakes first

Ask two questions before you start: what breaks if this is wrong, and can it be undone cheaply?
Then pick one level.

| Level | Typical work | Effort | Verification |
|---|---|---|---|
| Mechanical and reversible | rename, format, boilerplate, a typo, a lookup | fast, low | a glance at the diff |
| Ordinary code change with tests | a feature slice, a bug fix, a refactor with coverage | medium | run the tests that cover the change |
| High stakes | auth, security, money, production data, or a claim the user will act on unchecked | high | check it directly before you report |

## 2. What each level asks of you

- **Mechanical:** act, do not deliberate. Skip plans and long explanations. A wrong result is
  cheap to see and cheap to revert.
- **Ordinary:** state the goal, make the change, run the relevant checks and read their output.
  Do not claim success from a command you did not run.
- **High stakes:** slow down. Read the real code or data instead of recalling it. Reproduce the
  behaviour, run the exact command, query the actual state, and report only what you saw. Say what
  you could not verify. A worker on this level gets a high effort and you read its diff yourself.

## 3. Rules of thumb

- When two levels fit, take the higher one.
- A claim the user will act on without checking is high stakes, even if the task looked small.
- Do not raise effort to look careful. Extra effort on a mechanical task is wasted tokens.
- Do not lower effort to save tokens on a high-stakes task. A wrong answer costs more than the check.
- Changing level mid-task is fine: if a small edit turns out to touch auth, move up.

## 4. Handing it to a worker

State the level in the brief and name the check that proves it: the exact command that must exit 0,
or the file and line a claim rests on. Pick the effort for the spawn yourself, using the
`downshift-orchestrator` skill and the `providers` tool for what the harness offers.

Inspired by an MIT-licensed skill, github.com/mmc7676/skills.

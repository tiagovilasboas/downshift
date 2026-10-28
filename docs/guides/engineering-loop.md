# Downshift Engineering Loop Guide

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

`downshift loop` executes an autonomous engineering cycle implementing the BDI (Beliefs-Desires-Intentions) execution loop:
`Plan -> Implement -> Sensor -> Cascade -> Accept`

## How it works

1. **Plan & Implement:** Dispatches subagents to plan and implement the specified task.
2. **Sensor Check:** Executes automated verification tests or scripts (e.g. `downshift compactor -- go test ./...`).
3. **Dead-Ends Sensor & Failure Cascade:** If the verification sensor fails, the failure is recorded in `~/.downshift/dead_ends.json` with an atomic directory lock (`dead_ends.lock`). Subsequent retry spawns automatically auto-upgrade the model tier.
4. **Acceptance:** On success, git commit diffs are recorded as `used` code, and discarded branch work is tracked.

## Usage

```bash
# Execute an autonomous engineering loop on a task
downshift loop --task="Implement feature X with unit tests" --sensor="go test ./..."

# With explicit tier and model constraints
downshift loop --task="Fix issue Y" --sensor="make check" --model="claude-sonnet-5-5"
```

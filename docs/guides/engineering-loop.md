# Downshift loop guide

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

`downshift loop` is a **sensor retry loop**. It does not plan, write code or
spawn subagents; the harness (or you) does the work. The loop runs your check,
records the outcome, and feeds the failure record that the routing cascade
reads on the next spawn of the same task.

## What one run does (`internal/loop/runner.go`)

For each iteration, up to `--iterations` (default 3) and within `--timeout`:

1. Reads `dead_ends.json` to see whether the same task failed in this session
   in the last 15 minutes (`cascade_active` in the result).
2. Runs the sensor command, if one is given. Allowlisted commands go through
   the compactor; others run directly, without a shell.
3. **Sensor fails:** records the failure for `(session, task)` with the model
   from `--model` (or `<harness>-small` when none is given), then retries.
   After the last iteration it stops with a tripwire message.
4. **Sensor passes (or no sensor):** with `--session`, records an `used`
   acceptance and clears the session's failure records; the loop ends.

The sensor is re-run unchanged on each iteration, so a retry passes only if
something outside the loop changed in between (for example, the agent fixed the
code). The failure record is what lets the next spawn of that task move up a
tier.

## Usage

```bash
downshift loop --task="fix the parser test" --sensor="go test ./internal/parser/..." --session=<id>
downshift loop --task="lint" --iterations=1 --json -- make check
```

Flags: `--task`, `--sensor`, `--iterations`, `--timeout`, `--session`,
`--model`, `--json`, and `-- <sensor-cmd>` as an alternative to `--sensor`.

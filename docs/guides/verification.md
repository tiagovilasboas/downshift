# Verification evidence

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Cost alone cannot show whether a cheaper subagent checked its work. On Claude
Code, `SubagentStop` reads the child's transcript and records the last recognized
test, lint or type-check command's result alongside its usage event.

```bash
downshift verification-report
# Read a specific prompt-free event export instead of the default log:
downshift verification-report --events=/path/to/events.jsonl
```

The command emits JSON grouped by tier, with usage-event counts, verification
coverage, priced-call counts and catalog-priced cost. It reads the event log;
it does not run checks or change routing.

| Value | What the sensor observed |
|---|---|
| `passed` | The last recognized check's matching tool result had `is_error: false` |
| `failed` | The last recognized check's matching tool result had `is_error: true` |
| `none` | No recognized completed check was found, or the transcript could not be read safely |
| `unrecorded` | The usage event has no verification field, for example an older record |

The allowlist includes commands such as `go test`, `go vet`, `pytest`, `npm test`,
`tsc` and `eslint`. Unrecognized commands do not count. The sensor pairs Bash
tool calls with their results and keeps the last recognized result; it does not
interpret assertions, shell output or the scope of the tests. A wrapper that
swallows failures can produce a misleading success signal. Command text and
output are not persisted by this sensor.

**Limits:** `passed` is not task acceptance, security approval or proof of
correctness. `none` does not prove that no verification happened. Coverage is
the share of usage events with a recognized verification field, not test
coverage. Costs are estimates from catalog prices, not invoices. Other harnesses
have no equivalent validated child-usage sensor today.

Guia: interpret these values using this contract (inferencial, behaviour).
Sensor: transcript matching and report counters (computacional, behaviour).
Code: `internal/adapters/claudecode/verification.go`, `internal/childend/record.go`
and `cmd/downshift/verification.go`.

For actual delivery quality and routed-versus-control comparisons, follow the
[evidence ladder](evidence.md) and [benchmark protocol](benchmark-protocol.md).

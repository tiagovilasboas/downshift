# Synthetic MCP fixture

`python3 scripts/mcp-eval/fixture.py --case C03 --instructions /tmp/arm.txt --log /tmp/calls.jsonl` starts an isolated stdio JSON-RPC MCP fixture. It serves the provided arm text at initialization and logs synthetic tool calls, arguments and results. Start a new process and use a distinct log per run. Logs can contain the supplied synthetic prompts and should stay local.

Cases C03–C12 use simulated workers and test data; the fixture never starts processes, edits files, or accesses external services. C01 and C02 depend on native harness capabilities and require a separate harness fixture. Tool descriptions, argument schemas and worker responses are simplified: this is an exploratory behavioural fixture, not a protocol-conformance replacement for the production server. A/B runs must use this identical fixture and vary only the instruction file. The fixture itself does not evaluate a model, perform scoring, record usage, or prove non-regression.

Run its tests with `python3 -m unittest discover -s scripts/mcp-eval -p 'test_fixture.py'`.

# Launch and compare

`python3 scripts/mcp-eval/launch.py --case C03 --model MODEL --arm A --repetition 1 --instructions docs/evaluation/mcp-instructions/arm-a-read.txt --fixture scripts/mcp-eval/fixture.py --output /private/new-run` starts one ephemeral Codex session in a disposable workspace; `--output` must not already exist and holds private logs. The launcher snapshots and hashes fixture/instruction files, records elapsed time, and never scores the run. The candidate B text is experimental and is not used by the production server. Both texts apply only to read-worker mode. C01/C02 need separate native fixtures.

Score each run independently from its tool events. To compare, supply a JSON array containing `case`, `arm`, `repetition`, `accepted` (boolean or null), and `critical_failure` (boolean) to `python3 scripts/mcp-eval/compare.py docs/evaluation/mcp-instructions/cases.json records.json`. Missing or unscored runs stay incomplete; baseline failures and candidate regressions do not pass. This tool does not calculate token usage or cost.

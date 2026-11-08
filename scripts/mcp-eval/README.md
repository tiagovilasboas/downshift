# Synthetic MCP fixture

`python3 scripts/mcp-eval/fixture.py --case C03 --instructions /tmp/arm.txt --log /tmp/calls.jsonl` starts an isolated stdio JSON-RPC MCP fixture. It serves the provided arm text at initialization and logs synthetic tool calls, arguments and results. Start a new process and use a distinct log per run. Logs can contain the supplied synthetic prompts and should stay local.

Cases C03–C12 use simulated workers and test data; the fixture never starts processes, edits files, or accesses external services. C01 and C02 enable native multi-agent tools and include a disposable source file and answer file. A C01 pilot must first prove that ordered native-spawn events are observable in the CLI trace; C01/C02 remain unevaluated until native event coverage is demonstrated, including the absence claim in C02. The synthetic provider advertises the model passed to the launcher; it does not prove the harness supports that model for a child agent. Tool descriptions, argument schemas and worker responses are simplified: this is an exploratory behavioural fixture, not a protocol-conformance replacement for the production server. A/B runs must use this identical fixture and vary only the instruction file. The fixture itself does not evaluate a model, perform scoring, record usage, or prove non-regression.

Run its tests with `python3 -m unittest discover -s scripts/mcp-eval -p 'test_fixture.py'`.

# Launch and compare

`python3 scripts/mcp-eval/launch.py --case C03 --model MODEL --arm A --repetition 1 --instructions docs/evaluation/mcp-instructions/arm-a-read.txt --fixture scripts/mcp-eval/fixture.py --output /private/new-run` starts one ephemeral Codex session in a disposable workspace; `--output` must not already exist and holds private logs. The launcher snapshots and hashes fixture/instruction files, records elapsed time, and never scores the run. The candidate B text is experimental and is not used by the production server. Both texts apply only to read-worker mode. Native C01/C02 pilots may spawn agents; account for them under the session concurrency limit.

Score each run independently from its tool events. To compare, supply a JSON array containing `case`, `arm`, `repetition`, `accepted` (boolean or null), and `critical_failure` (boolean) to `python3 scripts/mcp-eval/compare.py docs/evaluation/mcp-instructions/cases.json records.json`. Missing or unscored runs stay incomplete; baseline failures and candidate regressions do not pass. This tool does not calculate token usage or cost.

// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

// maintainerHelp covers the commands for people who train and evaluate the
// classifier. They need labelled data that ships outside this repository.
var maintainerHelp = map[string]string{
	"harness2": `downshift harness2 — Local lab for recursive harness maturation

Usage:
  downshift harness2 matrix [--json]
  downshift harness2 lab [--json]
  downshift harness2 eval <dataset.json> [--json]

Flags:
  --json    Print machine-readable JSON

Completed harnesses are the only mutation targets. Functional, Beta and Experimental stay observation targets. The public dataset is a smoke fixture, not a promotion gate. The lab writes benchmark/harness2-lab/receipt.json in the checkout.

Examples:
  downshift harness2 matrix
  downshift harness2 lab

Next: commit and push the receipt so the remote checkout matches this lab run
`,
	"benchmark": `downshift benchmark — Run the classifier on a labelled dataset: confusion matrix and false-downshift rate

Usage:
  downshift benchmark <dataset.json> [--compare] [--report] [--gate] [--min-tier-accuracy=0.60]

Flags:
  --compare                   Legacy vs CapabilityRouter v2, side by side
  --report                    Print the report as JSON
  --gate                      Apply all gates: tier accuracy, FRONTIER->SMALL = 0, FRONTIER->MID max
  --min-tier-accuracy=X       Minimum tier accuracy, 0 to 1 (default 0.60)
  --max-frontier-to-mid=X     With --gate: maximum FRONTIER->MID rate (default 0.70)
  --candidate-weights=PATH    Candidate router weights to evaluate

The dataset is maintainer data; it is not in this repository.

Examples:
  downshift benchmark tasks.json --compare
  downshift benchmark tasks.json --gate

Next: downshift train, if the weights need retraining
`,
	"train": `downshift train — Train the capability router v2 classifier on a labelled dataset

Usage:
  downshift train <dataset.json> [--output=PATH] [--validation-split=0.2] [--epochs=N] [--learning-rate=X]
  downshift train --from-events

Flags:
  --output=PATH            Write the weights here
  --validation-split=X     Share of the data held out for validation (0 to 1)
  --epochs=N               Training passes
  --learning-rate=X        Step size
  --from-events            Train from engineer-reviewed local feedback instead of a dataset

Examples:
  downshift train tasks.json --output=weights.json
  downshift train --from-events

Next: downshift benchmark <dataset.json> --gate, to check the new weights
`,
	"eval-outcome": `downshift eval-outcome — Outcome eval: executable checks, small vs frontier pass rate

Usage:
  downshift eval-outcome --verify
  downshift eval-outcome --record --tier=small|mid|frontier --model=ID --solver='<cmd>'
  downshift eval-outcome --report [--write=FILE | --check=FILE]

Flags:
  --verify          Offline: every stub fails, every reference passes, recorded results reproduce
  --record          Solve every task with --solver (makes model calls; refused in CI)
  --tier=TIER       Tier label for --record: small, mid or frontier
  --model=ID        Model id recorded with --record
  --solver='CMD'    Shell command for --record: prompt on stdin, answer on stdout
  --report          Print the generated README block
  --write=FILE      With --report: rewrite the marked block in FILE
  --check=FILE      With --report: exit 1 if the marked block in FILE is stale
  --tasks=DIR       Task directory
  --runs=DIR        Recorded runs directory

--verify runs the task and solution Go code with your privileges and no sandbox.

Examples:
  downshift eval-outcome --verify
  downshift eval-outcome --report --check=README.md

Next: downshift eval-adapt --runs=DIR, for the adaptive memory efficacy
`,
	"eval-adapt": `downshift eval-adapt — Offline adapt memory efficacy from recorded tier runs (maintainer)

Usage:
  downshift eval-adapt --runs=DIR [--tasks=DIR] [--price-small=in,out] [--price-mid=in,out] [--price-frontier=in,out]

Flags:
  --runs=DIR              Recorded runs directory (required)
  --tasks=DIR             Task directory
  --price-small=in,out    Small tier USD per 1M tokens (enables the cost report)
  --price-mid=in,out      Mid tier USD per 1M tokens
  --price-frontier=in,out Frontier tier USD per 1M tokens
  --ref-in=N              Reference request input tokens per attempt (default 20000)
  --ref-out=N             Reference request output tokens per attempt (default 1000)
  --fail-penalty=USD      Cost of one failed attempt beyond tokens (default 0: escalation is free)

Examples:
  downshift eval-adapt --runs=runs/ --price-small=0.1,0.4

Next: downshift eval-outcome --report, to compare with the outcome eval
`,
	"shadow-report": `downshift shadow-report — Candidate observations vs reviewed labels, plus the E5 shadow summary

Usage:
  downshift shadow-report [--events=PATH] [--telemetry=PATH] [--text]

Flags:
  --events=PATH      Loop events file (JSONL)
  --telemetry=PATH   Telemetry events file (JSONL)
  --text             Plain text instead of JSON

Examples:
  downshift shadow-report --text

Next: downshift verification-report, for the verification sensor
`,
	"verification-report": `downshift verification-report — Verification outcomes per tier: coverage and token calls

Usage:
  downshift verification-report [--events=PATH]

Flags:
  --events=PATH   Events file (JSONL); default: the usual events log

Output is JSON.

Examples:
  downshift verification-report

Next: downshift shadow-report, for the candidate labels
`,
}

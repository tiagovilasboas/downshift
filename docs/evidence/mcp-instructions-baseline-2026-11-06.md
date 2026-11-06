<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# MCP instruction baseline — 2026-11-06

A local build of the working tree was started as `downshift mcp serve`. One JSON-RPC `initialize` followed by one `tools/list` returned 1,868 characters of server instructions and 13 tools with 7,853 characters of descriptions in total. None of the tool descriptions contained the initial sentence of the server instructions. This is one observation of one configuration, measured in characters, not model tokens. It includes no model task outcomes, retry counts, latency distribution, or cost data.

The Codex client exposed the server instructions repeatedly alongside individual tool descriptions in this session; the wire result above does not itself explain how the client presents them. Shortening individual descriptions cannot be assumed to eliminate that repetition. Preserve guidance for clients that load tools on demand.

Before changing instructions, freeze a representative suite with the same harness, model, settings and tasks in both arms. Include delegation decisions, provider lookup before each spawn, worker report trust, collect/ack, cancel and release, follow-up versus new worker, write-mode restrictions, and disabled workers. Record acceptance and critical failures per task, input/output tokens, latency, retries and human corrections. Compare against the unchanged baseline; do not accept lost critical behaviour for lower token counts. Run repeated trials and report sample size and failures. Follow [benchmark protocol](../guides/benchmark-protocol.md) for cost per accepted task and privacy constraints. No behavioural non-regression or savings is claimed from the character count.

Reproduce against each built executable with `python3 scripts/measure-mcp-surface.py /path/to/downshift`. The script emits per-tool and aggregate character and UTF-8 byte counts as JSON; compare like-for-like worker configuration and harness environment. It does not collect task text or measure tokens. Two consecutive runs against the same local binary produced identical measurements.

## Exploratory fixture pilot

One C03 run per arm was attempted using the same model and synthetic read-worker fixture. Both runs reached providers, delegated two read workers, collected and acknowledged results, and released workers; the candidate also used follow-ups. An earlier pilot failed at the client approval boundary and another used an earlier fixture revision; neither is a comparable result. The two final pilot runs were not independently scored and the initial A launcher did not record fixture hashes. This is one case, one run per arm, with simplified tool descriptions and synthetic workers; it cannot establish non-regression, token savings or production performance. No cost or token figures were collected. The candidate text is experimental and is not enabled in the product.

<!-- Copyright (c) 2026 Tiago de Carvalho Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Multilingual routing checks from Codex, 2026-10-16

harness-downshift by Tiago de Carvalho Vilas Boas
https://github.com/tiagovilasboas/downshift

## Scope

The source checkout was built into a temporary binary and exercised through the
Codex adapter with isolated state and event logs. These checks prove the hook
wire response and telemetry, not the model selected by a native Codex child.
Control sampling, graph context and MiniLM were disabled for this fixture run.

| Case | Wire result | Evidence |
|---|---|---|
| Spanish risk with an English rename verb | Allow without updated input | `UNSUPPORTED_LANGUAGE`, held unconfident downshift |
| English variable rename | Model rewritten to the catalog small tier, low effort | Sibling tool input fields preserved |
| Portuguese permission check | Allow without updated input | Risk category, risk floor and held downshift |

The fixture used catalog-known `gpt-6-sol` as baseline and an explicit session
allowlist. A separate probe using the current session's `gpt-6.1-sol` exposed a
catalog gap: the baseline was recorded as unknown. Do not treat that probe as
evidence that inherited model discovery works for this model.

## Native runtime limits

Read-only `doctor --json` observed recent Codex hook events in this session.
It found no Codex completion records from `PostToolUse`. Its historical
rewrite-honored counts do not establish which model these four agents ran.

Hooks reference `~/go/bin/downshift`; the executable on PATH is a different,
older Homebrew-path binary. Tests use the freshly built checkout. No installed
binary or harness configuration was changed as part of these checks.

## Guia and sensor

| Concern | Guia | Sensor | Eixo |
|---|---|---|---|
| Language coverage | Coverage and abstention contract in README and complexity weights (inferencial) | Full-input guard and cross-harness regression tests (computacional) | behaviour |
| Explainable abstention | Stable reason-code contract (inferencial) | Shared event code, stats/why and privacy assertions (computacional) | maintainability |
| Shared harness behavior | Adapter layering contract (inferencial) | Shared Route and all-adapter telemetry tests (computacional) | architecture fitness |
| Deployment evidence | Distinguish protocol output from executor acknowledgment (inferencial) | Fresh binary fixtures and native doctor observations (computacional) | behaviour |

The documentation is the guia; unit/integration tests and CI are the sensor.
Runtime reason-code counts provide continuous feedback without storing prompts.

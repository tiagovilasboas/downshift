<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Kiro IDE and KiroCrew: what the local binary analysis found (summary)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift-ai

**Date:** 2026-11-10. **Analysed:** Kiro.app v1.1.787 (kiroAgent extension) and KiroCrew.app v0.11.2.

The full analysis of both applications, with the code paths read, is kept in the maintainer's private `downshift-labs` repository (`reverse/kirocrew/findings/F9-kiro-ide-and-kirocrew-binary-analysis.md`). This page keeps only the conclusion Downshift relies on.

| Harness | PreToolUse hooks | `updatedInput.model` | Where the model is chosen |
|---------|------------------|----------------------|---------------------------|
| Kiro IDE | exit 0 or 2, plus `permissionDecision: ask` | not supported | design time (agent specs) |
| KiroCrew | script hooks | not supported | configuration (`model_scope`) |
| Claude Code (reference) | yes | yes | runtime rewrite |

**Conclusion:** neither Kiro IDE nor KiroCrew can rewrite a subagent's model at runtime, so Downshift observes there and does not rewrite. See [Kiro IDE: how Downshift works here](../guides/kiro-ide.md) and the [harness matrix](../reference/harness-matrix.md).

The live-probe evidence for the Kiro IDE support tier is in [kiro-observation-only-rea-2026-11-10.md](kiro-observation-only-rea-2026-11-10.md).

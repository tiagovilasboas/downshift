---
name: downshift-layering
description: Use before adding or changing a Downshift feature, adapter, hook, command or policy. Enforces the layering rule so no change serves only one harness.
---

# Downshift layering

Rule (AGENTS.md): behaviour in shared layers; adapters only parse, declare `HarnessCapabilities` and emit the wire response; harness facts are capability data; test at the shared layer.

Before coding, read [docs/contrib/layering.md](../../../docs/contrib/layering.md) and run its two checklists. If you are editing more than one adapter for the same logic, stop and move the logic down.

This file is a Claude Code bridge to the same text every other harness reads in AGENTS.md.

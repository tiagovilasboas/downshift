<!-- Copyright (c) 2026 Tiago de Carvalho Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# H14: unsafe confident recommendations (M3 baseline)

Source: `downshift-labs/eval/multilang/baseline.json` on fixture `d4213e51…` (170 rows).  
Definition: `confident == true` and recommended `tier` strictly below label `expected_tier`.

| ID | Lang | Expected | Got | Class | Risk | Notes |
|----|------|----------|-----|-------|------|-------|
| parallel-14-pt-BR | pt-BR | frontier | mid | MEDIUM | yes | Risk prompt; confident mid |
| parallel-17-en | en | frontier | mid | MEDIUM | yes | Risk prompt; confident mid |
| mixed-01 | mixed | frontier | small | TRIVIAL | yes | EN trivial + foreign risk (no `unsupported_language`) |
| mixed-11 | mixed | frontier | small | TRIVIAL | yes | Same pattern |
| mixed-27 | mixed | frontier | small | TRIVIAL | yes | Same pattern |
| delegation-16 … delegation-20 | en | frontier | mid | MEDIUM | yes | Long EN delegation briefs; risk labelled but class stays MEDIUM |

**Buckets:** (1) mixed-language TRIVIAL + risk — language guard gap when `unsupported_language` is false; (2) risk + confident MEDIUM without floor to frontier; (3) PT risk parity (`parallel-14-pt-BR`).

Do not tune on the five legacy benchmark splits. Re-measure on M3 after each rule change.

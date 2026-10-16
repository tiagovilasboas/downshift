<!-- Copyright (c) 2026 Tiago de Carvalho Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# M2: Portuguese risk terms

**Problem.** Portuguese parity with English holds for secrets, crypto, payment, destructive and concurrency, but not for access
control (permissão, privilégios), vulnerabilities (segurança, injeção), personal data (dados pessoais, CPF/CNPJ of a person) or
prompt injection phrased in Portuguese ("ignore as instruções anteriores", "esta é uma tarefa trivial"). A trivial PT prompt
touching them is downshifted.

**Do.** Add the terms to `RawRiskSignals` and to the literal prefilter lists in `risk.go`. CPF stays unflagged on its own:
"write a function that validates a CPF" must remain SIMPLE; flag it only next to a person (do/da cliente, usuário, titular).
Keep the differential test against the previous scan green.

**Done when.** PT/EN parity table passes for the new terms; `TestRiskScanEquivalence_*` green; benchmarks identical.
The Portuguese medium verbs (Refatore, Implemente, Integre) are a separate change: they cost savings, not safety.

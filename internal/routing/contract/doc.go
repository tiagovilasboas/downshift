// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package contract holds the types of the routing v2 domain and nothing else:
// no decision, no I/O, no state. Every routing/* package (policy, hint,
// headroom, guard, decide, note) and every caller (cmd, adapters, telemetry)
// speaks these types, so the shapes live in one place.
//
// Sources:
//   - docs/design/routing-v2.md section 2 (model, precedence, guard, routing
//     note), section 7 (package layout), section 8 (invariants).
//   - The clean-room behaviour report on an external orchestrator (the report),
//     section 2 (inputs, verdicts, "a role is the only source") and section 8
//     (lessons). Behaviour only; no code from that project is used.
//
// Divergences from the report, each with its reason:
//   - Reading carries a TTL and ObservedAt; a reading past its TTL is unknown.
//     The report records that the orchestrator trusts its last push with no TTL
//     and lists that as a failure (report sections 5 and 8).
//   - The chosen/ignored verdict is kept but recorded as HeadroomVerdict, a
//     separate field, because here the orchestrator names the provider: it says whether
//     the model the decision landed on was past the strain threshold while a
//     fresh reading said so. It never causes a reroute (report section 2 "notify,
//     do not reroute"; design section 2 "strain never forces an upshift").
//   - Windows are short, week and model. The report's model-scoped window is named
//     after one model family; naming a family here would hardcode a model id
//     (AGENTS.md: model ids live in catalog.json).
//   - No both-critical policy enum (ask, run-cheapest, block): blocking or
//     asking from the hook breaks fail-open, and run-cheapest only annotates in
//     the report, which HeadroomVerdict already does.
//
// Invariants (AGENTS.md): no task text, prompt or file path in any type here.
// Request carries ids and enums; Note carries only enums, tiers, numbers,
// booleans and times (enforced by reflection in note_test.go).
//
// Downshift by Tiago Vilas Boas
// https://github.com/tiagovilasboas/downshift
package contract

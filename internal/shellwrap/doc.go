// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package shellwrap is the harness-agnostic domain for the opt-in exec wrapper.
//
// It owns two things:
//
//  1. The adapter contracts that each harness implements once, in its own
//     adapter package (internal/adapters/<harness>/shell.go).
//
//  2. The routing policy (Handle) that reads those contracts and decides
//     whether to rewrite the command in place (RewriteAdapter) or advise a
//     retry via stderr/exit-2 (AdviseAdapter).
//
// The cmd/ orchestrator delegates to Handle and acts on the Result; it never
// contains routing logic. The policy packages (internal/execwrap,
// internal/core) are upstream of this package and must not import it.
package shellwrap

// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build race

package testrace

// Enabled reports whether the race detector is on.
const Enabled = true

// factor is how much slower code runs under -race; wall-clock budgets scale by it.
const factor = 10

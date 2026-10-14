// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build race

package main

// raceEnabled reports whether the race detector is on; it slows the
// session reader enough that wall-clock bounds do not hold.
const raceEnabled = true

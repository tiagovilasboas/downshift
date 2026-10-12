// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build race

package core

// raceEnabled reports whether the race detector is on; it slows the
// classifier by about 10x, so wall-clock bounds do not hold.
const raceEnabled = true

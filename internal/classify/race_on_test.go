// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build race

package classify

// raceEnabled reports whether the race detector is on. Wall-clock bounds
// in this package do not hold under -race.
const raceEnabled = true

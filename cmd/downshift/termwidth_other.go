// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !(darwin || linux)

package main

import "os"

// ttyColumns reports no width where the terminal query is not implemented: the
// default width applies.
func ttyColumns(*os.File) int { return 0 }

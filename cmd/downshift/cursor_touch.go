// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/tiagovilasboas/downshift/internal/adapters/cursor"
)

// runCursorTouch records the composer model from any Cursor hook that carries
// the common schema (sessionStart, beforeSubmitPrompt, subagentStop). It always
// exits 0 and prints {} so sessionStart stays fire-and-forget.
func runCursorTouch(in io.Reader) int {
	const max = 1 << 20
	raw, err := io.ReadAll(io.LimitReader(in, max+1))
	if err != nil || len(raw) > max {
		fmt.Fprintln(os.Stdout, `{}`)
		return 0
	}
	var ev cursor.Event
	if json.Unmarshal(raw, &ev) == nil {
		cursor.RememberFromHook(ev)
	}
	fmt.Fprintln(os.Stdout, `{}`)
	return 0
}

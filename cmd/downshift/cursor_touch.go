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

// cursorRemember is the seam tests replace to force a panic.
var cursorRemember = cursor.RememberFromHook

// runCursorTouch records the composer model from any Cursor hook that carries
// the common schema (sessionStart, beforeSubmitPrompt, subagentStop). It always
// exits 0 and prints {} so sessionStart stays fire-and-forget. It shares the
// hook runner's input guards (the 1 MiB cap and the read timeout of
// readHookPayload) and recovers from a panic, so it fails open like every hook.
func runCursorTouch(in io.Reader, out io.Writer) (rc int) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintln(os.Stderr, "downshift: internal error, response unchanged")
			fmt.Fprintln(out, `{}`)
			rc = 0
		}
	}()
	const max = 1 << 20
	raw, err := readHookPayload(in, max+1)
	if err == nil && len(raw) <= max {
		var ev cursor.Event
		if json.Unmarshal(raw, &ev) == nil {
			cursorRemember(ev)
		}
	}
	fmt.Fprintln(out, `{}`)
	return 0
}

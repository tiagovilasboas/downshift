// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
)

// dciRemovedNotice is what `downshift dci <anything but hook>` prints now that
// code intelligence is gone.
const dciRemovedNotice = "downshift: code intelligence (dci) was removed; this command does nothing."

// runDCIRemoved keeps the `dci` command name answering after code intelligence
// was removed. A harness configured by an older setup still runs
// `downshift dci hook <harness>` on every prompt submit, and in Claude Code a
// non-zero exit there blocks the user's prompt. So the hook form stays silent
// and exits 0; every other form says the feature is gone, also with exit 0.
func runDCIRemoved(args []string, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "hook" {
		return 0
	}
	fmt.Fprintln(stderr, dciRemovedNotice)
	return 0
}

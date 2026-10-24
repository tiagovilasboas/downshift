// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package shellwrap

import "fmt"

// AdviseText is the one line a harness relays to the model when a shell call
// of a compressing family should be run again through `downshift compact`.
// Every AdviseAdapter formats its message with it, so the wording lives here.
func AdviseText(wrappedCommand string) string {
	return fmt.Sprintf("downshift: this command prints a long log. Run it again exactly as: %s", wrappedCommand)
}

// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import "bytes"

// setupOwnedPhrase is written into every whole file setup generates: wrapper
// scripts and the dedicated steering file. A file without it is one a person
// has taken over, so setup will not overwrite it and --undo will not remove it.
const setupOwnedPhrase = "Written by `downshift setup`"

// setupOwnedNote carries the phrase in a markdown file, as an HTML comment.
const setupOwnedNote = "<!-- " + setupOwnedPhrase + ", which rewrites it. Edit nothing here. -->\n"

// setupOwns reports whether content carries the ownership marker.
func setupOwns(content []byte) bool {
	return bytes.Contains(content, []byte(setupOwnedPhrase))
}

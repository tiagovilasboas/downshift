// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package configfile

import "bytes"

// OwnedPhrase is written into every whole file setup generates: wrapper
// scripts and the dedicated steering file. A file without it is one a person
// has taken over, so setup will not overwrite it and --undo will not remove it.
const OwnedPhrase = "Written by `downshift setup`"

// OwnedNote carries the phrase in a markdown file, as an HTML comment.
const OwnedNote = "<!-- " + OwnedPhrase + ", which rewrites it. Edit nothing here. -->\n"

// Owns reports whether content carries the ownership marker.
func Owns(content []byte) bool {
	return bytes.Contains(content, []byte(OwnedPhrase))
}

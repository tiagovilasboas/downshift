// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "github.com/tiagovilasboas/downshift-ai/internal/compressor"

// searchFamilies print "path:line:text" rows.
var searchFamilies = map[Family]bool{FamilyGrep: true, FamilyGitGrep: true, FamilyRg: true}

// capSearchFamily is the row cut of a plain search the balanced cut leaves
// whole (output at or under the threshold): the first rows of each file via
// compressor.CapSearchRows, the raw output stored so every dropped row
// comes back with `downshift retrieve`. It reports false, and the caller
// keeps shown, when family is not a search, the run failed (exit above 1),
// keep is nil, no row is dropped, the cut with its marker is not shorter than
// shown, or the raw output could not be stored. A panic counts as false.
func capSearchFamily(keep func([]byte) (string, bool), family Family, exitCode int, raw, shown []byte) (res []byte, ok bool) {
	defer func() {
		if recover() != nil {
			res, ok = nil, false
		}
	}()
	if keep == nil || !searchFamilies[family] || exitCode < 0 || exitCode > 1 || len(raw) == 0 {
		return nil, false
	}
	body, applied := compressor.CapSearchRows(raw)
	if !applied || len(body)+len(MarkerWithID(len(raw), len(body), longestHintID)) >= len(shown) {
		return nil, false
	}
	id, stored := keep(append([]byte(nil), raw...))
	if !stored {
		return nil, false
	}
	return append(append([]byte{}, body...), MarkerWithID(len(raw), len(body), id)...), true
}

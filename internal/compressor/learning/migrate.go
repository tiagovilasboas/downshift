// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package learning

// sanitiseEntries rewrites legacy entries (first three argv words, which may
// hold paths, URLs, credentials or KEY=value pairs) to the current signature
// shape. Entries with no safe command word are dropped; entries that now share
// a signature are merged. It reports whether anything changed.
func sanitiseEntries(store *Store) bool {
	changed := false
	out := make(map[string]*Entry, len(store.Entries))
	for key, e := range store.Entries {
		if e == nil {
			changed = true
			continue
		}
		sig, ok := cleanSignature(e.Signature)
		if !ok {
			changed = true
			continue
		}
		if len(sig.Words) != len(e.Signature.Words) {
			// Words were dropped: rekey under the hash of what remains.
			changed = true
			e.Signature, key = sig, sig.Hash
		}
		if prev, dup := out[key]; dup {
			mergeEntry(prev, e)
			changed = true
			continue
		}
		out[key] = e
	}
	store.Entries = out
	return changed
}

func mergeEntry(dst, src *Entry) {
	dst.Count += src.Count
	dst.TotalBytes += src.TotalBytes
	if src.FirstSeen != "" && (dst.FirstSeen == "" || src.FirstSeen < dst.FirstSeen) {
		dst.FirstSeen = src.FirstSeen
	}
	if src.LastSeen > dst.LastSeen {
		dst.LastSeen = src.LastSeen
	}
	if dst.Shape == nil && src.Shape != nil {
		dst.Shape, dst.SampleBytes = src.Shape, src.SampleBytes
	}
	if dst.Family == "" {
		dst.Family = src.Family
	}
}

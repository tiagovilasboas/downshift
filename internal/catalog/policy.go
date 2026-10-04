// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1
// Commercial use requires a licence — see LICENSE for terms.

package catalog

import (
	"fmt"
	"sort"
	"strings"
)

// mergeEntries merges a user override slice into a base slice.
//
// Rules:
//   - An override entry that shares the same (harness, id) key as a base
//     entry replaces that base entry entirely.
//   - Override entries with no matching base key are appended (sorted for
//     determinism).
//   - Base entries without a matching override are preserved unchanged.
//
// The result always contains at least the full base set, so the embedded
// catalog is the guaranteed safe fallback.
func mergeEntries(base, override []Entry) []Entry {
	byKey := make(map[string]Entry, len(override))
	for _, e := range override {
		if e.ID != "" && e.Harness != "" {
			byKey[entryKey(e)] = e
		}
	}

	merged := make([]Entry, 0, len(base)+len(override))
	seen := make(map[string]bool, len(base))

	for _, e := range base {
		key := entryKey(e)
		if replacement, ok := byKey[key]; ok {
			merged = append(merged, replacement)
			seen[key] = true
		} else {
			merged = append(merged, e)
		}
	}

	// Append net-new override entries in a deterministic order.
	var appended []Entry
	for key, e := range byKey {
		if !seen[key] {
			appended = append(appended, e)
		}
	}
	sort.Slice(appended, func(i, j int) bool {
		return entryKey(appended[i]) < entryKey(appended[j])
	})

	return append(merged, appended...)
}

// validateEntries checks the entry slice for two classes of policy violation:
//
//  1. Duplicate model IDs or aliases within the same harness.
//     A user override that introduces the same name twice is rejected so that
//     routing decisions remain deterministic.
//
//  2. Overlapping family prefixes within the same harness.
//     If family A is a prefix of family B (or vice-versa), the version-agnostic
//     lookup cannot distinguish them, so it is treated as a configuration error.
//
// Section/comment entries (empty ID or Harness) are silently skipped.
func validateEntries(entries []Entry) error {
	used := make(map[string]string)                // normalised "harness\x00name" → first id that claimed it
	families := make(map[string][]familyEffortTag) // normalised harness → family prefixes with effort tags

	for _, e := range entries {
		if e.ID == "" || e.Harness == "" {
			continue // section headers / comments
		}

		// Check for duplicate IDs and aliases.
		for _, name := range append([]string{e.ID}, e.Aliases...) {
			key := strings.ToLower(e.Harness + "\x00" + name)
			if prior, exists := used[key]; exists {
				return fmt.Errorf(
					"catalog: duplicate model or alias %q for harness %q (also registered by %q)",
					name, e.Harness, prior,
				)
			}
			used[key] = e.ID
		}

		// Check for overlapping family prefixes.
		//
		// Exception (Option A): overlapping families are allowed when both
		// entries carry an explicit, differing effort tag — they are effort
		// variants inside one family (e.g. opus high vs opus medium), resolved
		// via FamilyModelFor instead of prefix order. Same effort (or an
		// untagged entry) stays an error to keep lookup deterministic.
		if e.Family != "" {
			harnessKey := strings.ToLower(e.Harness)
			family := strings.ToLower(e.Family)
			eff := normalizeEffort(e.Effort)
			for _, prior := range families[harnessKey] {
				if strings.HasPrefix(family, prior.prefix) || strings.HasPrefix(prior.prefix, family) {
					if eff != "" && prior.effort != "" && eff != prior.effort {
						continue
					}
					return fmt.Errorf(
						"catalog: overlapping model families %q and %q for harness %q",
						family, prior.prefix, e.Harness,
					)
				}
			}
			families[harnessKey] = append(families[harnessKey], familyEffortTag{prefix: family, effort: eff})
		}
	}
	if err := rejectForeignCanonicalAlias(entries); err != nil {
		return err
	}
	return nil
}

// rejectForeignCanonicalAlias stops harness B from listing harness A's
// canonical id as an alias. Two harnesses may each own the same canonical
// id (their own catalog row). They may not borrow it.
func rejectForeignCanonicalAlias(entries []Entry) error {
	owners := make(map[string]map[string]struct{})
	for _, e := range entries {
		if e.ID == "" || e.Harness == "" {
			continue
		}
		if owners[e.ID] == nil {
			owners[e.ID] = make(map[string]struct{})
		}
		owners[e.ID][e.Harness] = struct{}{}
	}
	for _, e := range entries {
		if e.ID == "" || e.Harness == "" {
			continue
		}
		for _, alias := range e.Aliases {
			harnesses := owners[alias]
			if len(harnesses) == 0 {
				continue
			}
			if _, own := harnesses[e.Harness]; own {
				continue
			}
			return fmt.Errorf("catalog: alias %q on harness %q is another harness's canonical id", alias, e.Harness)
		}
	}
	return nil
}

// entryKey builds a canonical deduplication key for an entry.
func entryKey(e Entry) string { return e.Harness + "\x00" + e.ID }

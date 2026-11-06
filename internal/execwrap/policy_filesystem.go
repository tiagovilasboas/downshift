// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "strings"

// ════════════════════════════════════════════════════════════════════════════
// DOMAIN: Filesystem writes — mkdir, touch, cp, mv, rm (never recursive)
// ════════════════════════════════════════════════════════════════════════════

// Filesystem write families: tracked for safety, never compacted.
const (
	FamilyMkdir Family = "mkdir"
	FamilyTouch Family = "touch"
	FamilyCp    Family = "cp"
	FamilyMv    Family = "mv"
	FamilyRm    Family = "rm"
)

// filesystemWriteFamily detects filesystem write commands. A recursive rm in
// any spelling never matches.
func filesystemWriteFamily(argv []string) Family {
	if len(argv) == 0 {
		return FamilyNone
	}
	switch argv[0] {
	case "mkdir":
		return FamilyMkdir
	case "touch":
		return FamilyTouch
	case "cp", "gcp":
		return FamilyCp
	case "mv", "gmv":
		return FamilyMv
	case "rm", "grm":
		if rmRecursive(argv[1:]) {
			return FamilyNone
		}
		return FamilyRm
	}
	return FamilyNone
}

// rmRecursive reports whether rm arguments ask for a recursive delete in any
// spelling: a short-option cluster holding r or R (-r, -R, -Rf, -vrf), or a
// long option GNU rm reads as --recursive, which accepts any unambiguous
// prefix such as --rec. Every argument is checked, because GNU rm also reads
// options after an operand; an operand literally named -r is refused too,
// which only costs running that rm without the wrapper.
func rmRecursive(args []string) bool {
	for _, a := range args {
		switch {
		case strings.HasPrefix(a, "--"):
			if len(a) > 2 && strings.HasPrefix("--recursive", a) {
				return true
			}
		case strings.HasPrefix(a, "-"):
			if strings.ContainsAny(a[1:], "rR") {
				return true
			}
		}
	}
	return false
}

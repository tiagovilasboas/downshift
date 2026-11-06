// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strconv"
	"strings"
)

// semver is the part of a version string that orders releases: major, minor,
// patch and the pre-release suffix ("" for a release).
type semver struct {
	nums [3]int
	pre  string
}

// parseSemver reads "v1.2.3", "1.2.3-rc1" and Go pseudo-versions
// ("v0.8.2-0.2026...-abcdef", "v0.0.0-2026...-abcdef"). "dev", "(devel)" and
// empty strings are not versions.
func parseSemver(s string) (semver, bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	if i := strings.IndexByte(s, '+'); i >= 0 {
		s = s[:i]
	}
	core, pre, hasPre := strings.Cut(s, "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return semver{}, false
	}
	v := semver{}
	if hasPre {
		v.pre = pre
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return semver{}, false
		}
		v.nums[i] = n
	}
	return v, true
}

// compareVersions returns -1, 0 or 1, and false when either side is not a
// version (dev builds), so callers report "unknown" instead of a verdict.
func compareVersions(a, b string) (int, bool) {
	x, okA := parseSemver(a)
	y, okB := parseSemver(b)
	if !okA || !okB {
		return 0, false
	}
	for i := range x.nums {
		if x.nums[i] != y.nums[i] {
			if x.nums[i] < y.nums[i] {
				return -1, true
			}
			return 1, true
		}
	}
	return comparePre(x.pre, y.pre), true
}

// comparePre orders pre-release suffixes per semver 2.0 section 11: a release
// (no suffix) is above any pre-release; identifiers are compared left to
// right, numeric ones numerically and below alphanumeric ones, and a longer
// set wins when all earlier identifiers are equal.
func comparePre(a, b string) int {
	switch {
	case a == b:
		return 0
	case a == "":
		return 1
	case b == "":
		return -1
	}
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(as) && i < len(bs); i++ {
		if c := compareIdent(as[i], bs[i]); c != 0 {
			return c
		}
	}
	return cmpInt(len(as), len(bs))
}

func compareIdent(a, b string) int {
	na, errA := strconv.Atoi(a)
	nb, errB := strconv.Atoi(b)
	switch {
	case errA == nil && errB == nil:
		return cmpInt(na, nb)
	case errA == nil:
		return -1
	case errB == nil:
		return 1
	}
	return strings.Compare(a, b)
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

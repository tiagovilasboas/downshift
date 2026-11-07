// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"regexp"
	"strings"
)

// Matcher overlap for the `doctor` hook overlap check (doctor_hook_overlap.go):
// do two hook matchers select a tool in common? A matcher is a tool name, an
// a|b|c list, an anchored or grouped pattern, or empty / "*" for every tool.
// Nothing here names a harness.

// matcherWildcard reports a matcher that selects every tool.
func matcherWildcard(m string) bool {
	switch strings.TrimSpace(m) {
	case "", "*", ".*", "^.*$":
		return true
	}
	return false
}

// matcherNames lists the plain tool names a matcher spells out: the
// alternatives of a|b|c once anchors and one outer group are removed. An
// alternative that is itself a pattern (".*spawn_agent$") is not a name.
func matcherNames(m string) []string {
	m = strings.TrimSpace(m)
	if strings.HasPrefix(m, "(") && strings.HasSuffix(m, ")") {
		m = m[1 : len(m)-1]
	}
	var names []string
	for _, alt := range strings.Split(m, "|") {
		alt = strings.Trim(alt, "^$ ")
		if alt != "" && !strings.ContainsAny(alt, `.*+?[](){}\`) {
			names = append(names, alt)
		}
	}
	return names
}

// matchesName reports whether matcher m selects the tool called name.
func matchesName(m, name string) bool {
	if matcherWildcard(m) {
		return true
	}
	re, err := regexp.Compile("^(?:" + strings.TrimSpace(m) + ")$")
	if err != nil {
		return m == name
	}
	return re.MatchString(name)
}

// matchersOverlap reports whether some tool is selected by both matchers. It
// tests the plain names each matcher spells out against the other; two
// patterns with no plain name in common are treated as disjoint.
func matchersOverlap(a, b string) bool {
	if matcherWildcard(a) || matcherWildcard(b) {
		return true
	}
	for _, n := range matcherNames(a) {
		if matchesName(b, n) {
			return true
		}
	}
	for _, n := range matcherNames(b) {
		if matchesName(a, n) {
			return true
		}
	}
	return false
}

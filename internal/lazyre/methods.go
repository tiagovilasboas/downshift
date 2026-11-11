// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package lazyre

// The methods below forward to the compiled *regexp.Regexp; see its docs.

// MatchString reports whether s contains a match.
func (r *Regexp) MatchString(s string) bool { return r.Get().MatchString(s) }

// Match reports whether b contains a match.
func (r *Regexp) Match(b []byte) bool { return r.Get().Match(b) }

// FindString returns the leftmost match in s.
func (r *Regexp) FindString(s string) string { return r.Get().FindString(s) }

// FindStringSubmatch returns the leftmost match in s and its submatches.
func (r *Regexp) FindStringSubmatch(s string) []string { return r.Get().FindStringSubmatch(s) }

// FindSubmatch returns the leftmost match in b and its submatches.
func (r *Regexp) FindSubmatch(b []byte) [][]byte { return r.Get().FindSubmatch(b) }

// FindStringSubmatchIndex returns the index pairs of the leftmost match in s.
func (r *Regexp) FindStringSubmatchIndex(s string) []int {
	return r.Get().FindStringSubmatchIndex(s)
}

// FindAllString returns up to n successive matches in s (all if n < 0).
func (r *Regexp) FindAllString(s string, n int) []string { return r.Get().FindAllString(s, n) }

// FindAll returns up to n successive matches in b (all if n < 0).
func (r *Regexp) FindAll(b []byte, n int) [][]byte { return r.Get().FindAll(b, n) }

// FindAllSubmatch returns up to n successive matches in b with submatches.
func (r *Regexp) FindAllSubmatch(b []byte, n int) [][][]byte {
	return r.Get().FindAllSubmatch(b, n)
}

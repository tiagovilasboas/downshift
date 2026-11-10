// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package lazyre

// ReplaceAllString replaces matches in src with repl ($ expanded).
func (r *Regexp) ReplaceAllString(src, repl string) string {
	return r.Get().ReplaceAllString(src, repl)
}

// ReplaceAll replaces matches in src with repl ($ expanded).
func (r *Regexp) ReplaceAll(src, repl []byte) []byte { return r.Get().ReplaceAll(src, repl) }

// ReplaceAllStringFunc replaces matches in src with the return value of fn.
func (r *Regexp) ReplaceAllStringFunc(src string, fn func(string) string) string {
	return r.Get().ReplaceAllStringFunc(src, fn)
}

// ReplaceAllLiteralString replaces matches in src with repl, unexpanded.
func (r *Regexp) ReplaceAllLiteralString(src, repl string) string {
	return r.Get().ReplaceAllLiteralString(src, repl)
}

// Split slices s into substrings separated by the expression.
func (r *Regexp) Split(s string, n int) []string { return r.Get().Split(s, n) }

// FindStringIndex returns the index pair of the leftmost match in s.
func (r *Regexp) FindStringIndex(s string) []int { return r.Get().FindStringIndex(s) }

// FindAllStringSubmatch returns up to n successive matches in s with submatches.
func (r *Regexp) FindAllStringSubmatch(s string, n int) [][]string {
	return r.Get().FindAllStringSubmatch(s, n)
}

// SubexpIndex returns the index of the first subexpression named name.
func (r *Regexp) SubexpIndex(name string) int { return r.Get().SubexpIndex(name) }

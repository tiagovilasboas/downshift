// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package lazyre holds a regular expression that compiles on first use.
//
// Every hook call and every wrapped command starts the downshift binary, and a
// package-level regexp.MustCompile runs at init on every start whether the
// expression is used or not. A package-level lazyre.New only records the
// source; the first method call compiles it once (safe for concurrent use).
// The methods mirror *regexp.Regexp, so a call site reads the same.
//
// An invalid expression panics on first use instead of at init, so every
// package that declares them runs CompileAll in a test (see the startup guard
// in internal/archtest).
package lazyre

import (
	"regexp"
	"sync"
)

// Regexp is a regular expression compiled on first use.
type Regexp struct {
	once sync.Once
	expr string
	re   *regexp.Regexp
}

var (
	registryMu sync.Mutex
	registry   []*Regexp
)

// New records expr; it compiles on the first method call. expr must be valid
// (it panics on first use otherwise, as regexp.MustCompile would at init).
func New(expr string) *Regexp {
	r := &Regexp{expr: expr}
	registryMu.Lock()
	registry = append(registry, r)
	registryMu.Unlock()
	return r
}

// Get returns the compiled expression, compiling it on the first call.
func (r *Regexp) Get() *regexp.Regexp {
	r.once.Do(func() { r.re = regexp.MustCompile(r.expr) })
	return r.re
}

// String returns the source text without compiling it.
func (r *Regexp) String() string { return r.expr }

// CompileAll compiles every expression declared so far and returns how many
// there are. Tests call it so an invalid expression fails the suite, not a
// user's first run of the one command that uses it.
func CompileAll() int {
	registryMu.Lock()
	all := append([]*Regexp(nil), registry...)
	registryMu.Unlock()
	for _, r := range all {
		r.Get()
	}
	return len(all)
}

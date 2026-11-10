// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package classify

import (
	"regexp"
	"sync"
)

// signal is a weighted matcher for a complexity class.
type signal struct {
	re     *regexp.Regexp
	weight int
	class  Complexity
	domain string // empty for unchanged general signals
}

// signals are ordered from strongest to weakest evidence. We score every
// class and pick the top; ties break toward the higher complexity to stay
// safe (never underpower a task).
//
// Compiled on first use, not at package init: every hook call and every
// wrapped command starts this binary, and most never classify a task.
var signals = sync.OnceValue(compileSignals)

func compileSignals() []signal {
	out := make([]signal, 0, len(RawSignals))
	for _, r := range RawSignals {
		out = append(out, signal{
			re:     regexp.MustCompile(`(?i)` + r.Pattern),
			weight: r.Weight,
			class:  r.Class,
		})
	}
	return out
}

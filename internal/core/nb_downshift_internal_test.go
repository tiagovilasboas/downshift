// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/nbtier"
)

// The TRIVIAL downshift never fires below the risk floor, after a keyword
// matched, on a non-Medium result, or after a graphify escalation, even when
// NB itself says TRIVIAL.
func TestNBTrivialOpinion_Gates(t *testing.T) {
	t.Setenv(NBDownshiftEnv, "1")
	prompt := "check what changed" // stand-in; the gates are checked before NB
	noSignal := Classification{Complexity: Medium, NoSignal: true}
	for name, tc := range map[string]struct {
		cls       Classification
		escalated bool
	}{
		"keyword fired": {Classification{Complexity: Medium}, false},
		"not medium":    {Classification{Complexity: Complex, NoSignal: true}, false},
		"risk floored":  {Classification{Complexity: Medium, NoSignal: true, RiskFloor: true}, false},
		"graph":         {noSignal, true},
	} {
		if got := nbTrivialOpinion(prompt, tc.cls, tc.escalated); got.Would || got.Applied {
			t.Errorf("%s: %+v", name, got)
		}
	}
	// A risky prompt that NB still rates TRIVIAL stays put: the floor would
	// lift it, so the downshift must not go below it.
	risky := "run rm -rf"
	if ok, _ := nbtier.Trivial(risky); !ok {
		t.Skipf("precondition: NB does not rate %q TRIVIAL", risky)
	}
	if got := nbTrivialOpinion(risky, noSignal, false); got.Would {
		t.Errorf("risk prompt downshifted: %+v", got)
	}
}

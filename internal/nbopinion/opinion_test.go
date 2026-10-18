// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package nbopinion

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/nbtier"
)

func TestHooksMatchNBTier(t *testing.T) {
	if core.NBSuggestHook == nil || core.NBTrivialHook == nil {
		t.Fatal("init must register both hooks")
	}
	const prompt = "rename the variable"
	s := nbtier.Suggest(prompt, nbtier.Small)
	up, to, margin := core.NBSuggestHook(prompt, nbtier.Small)
	if up != s.Upshift || to != s.To || margin != s.Margin {
		t.Fatalf("suggest hook = %v %d %v, nbtier = %+v", up, to, margin, s)
	}
	ok, m := core.NBTrivialHook(prompt)
	wantOK, wantM := nbtier.Trivial(prompt)
	if ok != wantOK || m != wantM {
		t.Fatalf("trivial hook = %v %v, nbtier = %v %v", ok, m, wantOK, wantM)
	}
}

// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"fmt"
	"math"
	"testing"
)

func TestInControlGroup_ShareAccuracy(t *testing.T) {
	const n = 2000
	for _, share := range []float64{0.05, 0.10, 0.25, 0.5} {
		hit := 0
		for i := 0; i < n; i++ {
			if InControlGroup(fmt.Sprintf("%032x", uint64(i)*2654435761+12345), share) {
				hit++
			}
		}
		got := float64(hit) / n
		if math.Abs(got-share) > 0.01 {
			t.Errorf("share %.2f: observed %.4f, want within 1 pp", share, got)
		}
	}
}

func TestInControlGroup_DeterministicAndBounds(t *testing.T) {
	id := "0123456789abcdef0123456789abcdef"
	first := InControlGroup(id, 0.5)
	for i := 0; i < 50; i++ {
		if InControlGroup(id, 0.5) != first {
			t.Fatal("same id and share must always land on the same side")
		}
	}
	if InControlGroup(id, 0) || InControlGroup(id, -1) {
		t.Error("share <= 0 disables the control group")
	}
	if !InControlGroup(id, 1) {
		t.Error("share 1 puts every spawn in the control group")
	}
	if InControlGroup("", 1) {
		t.Error("an empty id cannot be assigned")
	}
	// Growing the share only adds spawns: a spawn in at 10% stays in at 50%.
	for i := 0; i < 500; i++ {
		id := fmt.Sprintf("%032x", i)
		if InControlGroup(id, 0.10) && !InControlGroup(id, 0.50) {
			t.Fatalf("id %s left the group when the share grew", id)
		}
	}
}

func TestControlShare_Env(t *testing.T) {
	cases := map[string]float64{
		"": DefaultControlShare, "0": 0, "0.25": 0.25, " 0.3 ": 0.3, "1": 1,
		"2": 1, "-1": 0, "abc": DefaultControlShare, "NaN": DefaultControlShare,
	}
	for in, want := range cases {
		t.Setenv(ControlShareEnv, in)
		if got := ControlShare(); got != want {
			t.Errorf("%q: ControlShare() = %v, want %v", in, got, want)
		}
	}
}

// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setupwizard

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/tiagovilasboas/downshift/internal/console/shared/diffview"
)

var update = flag.Bool("update", false, "rewrite golden files")

func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden (run with -update): %v", err)
	}
	if string(want) != got {
		t.Errorf("%s differs from golden:\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

func TestGoldenViews(t *testing.T) {
	for _, w := range []int{80, 60} {
		m := newModel(t, sampleOptions(nil))
		m, _ = m.Update(tea.WindowSizeMsg{Width: w, Height: 24})
		golden(t, fmt.Sprintf("w%d_1_harnesses", w), m.View())
		m = press(m, enter)
		golden(t, fmt.Sprintf("w%d_2_components", w), m.View())
		m = deliverPreview(t, m)
		golden(t, fmt.Sprintf("w%d_3_review", w), m.View())
		m = press(m, enter)
		golden(t, fmt.Sprintf("w%d_4_confirm", w), m.View())
	}
}

func TestGoldenReviewStates(t *testing.T) {
	m := newModel(t, sampleOptions(nil))
	m = press(m, enter)
	m, _ = m.Update(enter) // loading, result not delivered yet
	golden(t, "w80_3_review_loading", m.View())

	empty := newModel(t, sampleOptions(func(Selection) ([]diffview.File, error) { return nil, nil }))
	golden(t, "w80_3_review_empty", deliverPreview(t, press(empty, enter)).View())
}

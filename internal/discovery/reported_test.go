// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package discovery_test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/discovery"
	"github.com/tiagovilasboas/downshift/internal/session"
)

var t0 = time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)

func ttlDay(t *testing.T) {
	t.Helper()
	t.Cleanup(session.SetConfig(session.Config{DiscoveryTTL: 24 * time.Hour}))
}

func catIDs(cat *catalog.Catalog) (harness, small, big string) {
	harness = core.AllHarnessCaps()[0].Harness
	return harness, cat.ModelFor(harness, core.TierSmall).ID, cat.ModelFor(harness, core.TierFrontier).ID
}

func cachePath(t *testing.T) string { return filepath.Join(t.TempDir(), "discovered.json") }

func detected(ids ...string) discovery.Entry {
	return discovery.Entry{Source: "cli", FetchedAt: t0, Ranked: discovery.Ranked{IDs: ids}}
}

func TestMergeReported_WritesWhenEmpty(t *testing.T) {
	ttlDay(t)
	cat := catalog.Load()
	h, small, big := catIDs(cat)
	p := cachePath(t)
	r, err := discovery.MergeReported(p, h, []string{small, big}, t0, cat)
	if err != nil || !r.Written || r.Accepted != 2 || r.Reason != discovery.ReasonWritten {
		t.Fatalf("result %+v err %v", r, err)
	}
	e := discovery.Read(p).Harnesses[h]
	// claude-code is SessionFromCatalog: the reported ids are added to the
	// 4-model catalog stand-in the router already saw. small and big are both
	// in the stand-in, so the union stays the 4 stand-in models.
	if e.Source != "mcp" || !e.FetchedAt.Equal(t0) || len(e.Models) != 4 || len(e.IDs) == 0 {
		t.Fatalf("entry %+v", e)
	}
	if !hasModel(e, small) || !hasModel(e, big) {
		t.Fatalf("union dropped a reported id: %+v", e.Models)
	}
	if runtime.GOOS != "windows" {
		if st, _ := os.Stat(p); st.Mode().Perm() != 0o600 {
			t.Errorf("mode %v", st.Mode().Perm())
		}
	}
}

func TestMergeReported_FreshDetectionWins(t *testing.T) {
	ttlDay(t)
	cat := catalog.Load()
	h, small, big := catIDs(cat)
	p := cachePath(t)
	if err := discovery.Merge(p, map[string]discovery.Entry{h: detected(small)}); err != nil {
		t.Fatal(err)
	}
	r, _ := discovery.MergeReported(p, h, []string{small, big}, t0.Add(time.Hour), cat)
	if r.Written || r.Reason != discovery.ReasonFreshWins {
		t.Fatalf("result %+v", r)
	}
	e := discovery.Read(p).Harnesses[h]
	if e.Source != "cli" || len(e.IDs) != 1 || e.IDs[0] != small {
		t.Fatalf("detection changed: %+v", e)
	}
}

func TestMergeReported_StaleDetectionIsReplaced(t *testing.T) {
	ttlDay(t)
	cat := catalog.Load()
	h, small, _ := catIDs(cat)
	p := cachePath(t)
	_ = discovery.Merge(p, map[string]discovery.Entry{h: detected(small)})
	r, _ := discovery.MergeReported(p, h, []string{small}, t0.Add(25*time.Hour), cat)
	if !r.Written || discovery.Read(p).Harnesses[h].Source != "mcp" {
		t.Fatalf("stale not replaced: %+v", r)
	}
}

func TestMergeReported_DropsInvalidAndDuplicatesAndCaps(t *testing.T) {
	ttlDay(t)
	cat := catalog.Load()
	h, small, _ := catIDs(cat)
	ids := []string{small, small, "has space", "", "\x1b[31m", "-lead"}
	for i := 0; i < 100; i++ {
		ids = append(ids, fmt.Sprintf("opaque-%d", i))
	}
	p := cachePath(t)
	r, _ := discovery.MergeReported(p, h, ids, t0, cat)
	// Accepted counts the reported ids, capped at MaxReported. The stored
	// union adds the stand-in ids the 64 reported did not already contain
	// (sonnet, opus, fable; haiku == small is already in), so 64 + 3 = 67.
	if r.Accepted != discovery.MaxReported || len(discovery.Read(p).Harnesses[h].Models) != 67 {
		t.Fatalf("result %+v models %d", r, len(discovery.Read(p).Harnesses[h].Models))
	}
	none, _ := discovery.MergeReported(cachePath(t), h, []string{"a b", ""}, t0, cat)
	if none.Written || none.Reason != discovery.ReasonNoValidIDs {
		t.Fatalf("no-valid: %+v", none)
	}
}

func TestMerge_DiscoverReplacesMCP(t *testing.T) {
	ttlDay(t)
	cat := catalog.Load()
	h, small, big := catIDs(cat)
	p := cachePath(t)
	_, _ = discovery.MergeReported(p, h, []string{big}, t0, cat)
	_ = discovery.Merge(p, map[string]discovery.Entry{h: detected(small)})
	if e := discovery.Read(p).Harnesses[h]; e.Source != "cli" || e.IDs[0] != small {
		t.Fatalf("%+v", e)
	}
}

// hasModel reports whether the entry lists the id in its Models.
func hasModel(e discovery.Entry, id string) bool {
	for _, m := range e.Models {
		if m.ID == id {
			return true
		}
	}
	return false
}

// A one-id report on a SessionFromCatalog harness with a 4-model stand-in must
// leave 4+ models visible: the report widens, it never narrows. This is the
// 2026-10-31 bug (one sonnet report had narrowed claude-code to one model).
func TestMergeReported_AddsToCatalogStandIn(t *testing.T) {
	ttlDay(t)
	cat := catalog.Load()
	h := "claude-code"
	p := cachePath(t)
	// claude-sonnet-5-5 is in the stand-in; report it alone.
	r, _ := discovery.MergeReported(p, h, []string{"claude-sonnet-5-5"}, t0, cat)
	if !r.Written || r.Accepted != 1 {
		t.Fatalf("result %+v", r)
	}
	e := discovery.Read(p).Harnesses[h]
	if len(e.Models) < 4 {
		t.Fatalf("report narrowed the stand-in to %d models: %+v", len(e.Models), e.Models)
	}
	if !hasModel(e, "claude-sonnet-5-5") {
		t.Fatalf("reported id missing from union: %+v", e.Models)
	}
}

// A second report is added to what the first left, never replacing it.
func TestMergeReported_SecondReportAddsToFirst(t *testing.T) {
	ttlDay(t)
	cat := catalog.Load()
	h := "claude-code"
	p := cachePath(t)
	_, _ = discovery.MergeReported(p, h, []string{"opaque-one"}, t0, cat)
	r, _ := discovery.MergeReported(p, h, []string{"opaque-two"}, t0.Add(time.Minute), cat)
	if !r.Written {
		t.Fatalf("result %+v", r)
	}
	e := discovery.Read(p).Harnesses[h]
	if !hasModel(e, "opaque-one") || !hasModel(e, "opaque-two") {
		t.Fatalf("second report dropped the first: %+v", e.Models)
	}
	// The 4 stand-in models plus the two reported ids.
	if len(e.Models) != 6 {
		t.Fatalf("union = %d models, want 6: %+v", len(e.Models), e.Models)
	}
}

// A fresh real detection still wins untouched: no stand-in, no union.
func TestMergeReported_FreshDetectionWinsOverUnion(t *testing.T) {
	ttlDay(t)
	cat := catalog.Load()
	h := "claude-code"
	p := cachePath(t)
	if err := discovery.Merge(p, map[string]discovery.Entry{h: detected("claude-sonnet-5-5")}); err != nil {
		t.Fatal(err)
	}
	r, _ := discovery.MergeReported(p, h, []string{"claude-opus-5-5"}, t0.Add(time.Hour), cat)
	if r.Written || r.Reason != discovery.ReasonFreshWins {
		t.Fatalf("fresh detection did not win: %+v", r)
	}
	e := discovery.Read(p).Harnesses[h]
	if e.Source != "cli" || len(e.IDs) != 1 || e.IDs[0] != "claude-sonnet-5-5" {
		t.Fatalf("detection changed: %+v", e)
	}
}

// An id outside the catalog is kept as reported, but the catalog invents no
// other id beyond the stand-in it already serves.
func TestMergeReported_KeepsOutOfCatalogIDWithoutInventing(t *testing.T) {
	ttlDay(t)
	cat := catalog.Load()
	h := "claude-code"
	p := cachePath(t)
	r, _ := discovery.MergeReported(p, h, []string{"opaque-outsider"}, t0, cat)
	if !r.Written {
		t.Fatalf("result %+v", r)
	}
	e := discovery.Read(p).Harnesses[h]
	if !hasModel(e, "opaque-outsider") {
		t.Fatalf("out-of-catalog id dropped: %+v", e.Models)
	}
	// Exactly the 4 stand-in ids plus the one reported id: nothing invented.
	if len(e.Models) != 5 {
		t.Fatalf("union = %d models, want 5 (4 stand-in + 1 reported): %+v", len(e.Models), e.Models)
	}
}

func TestMergeReported_MCPDoesNotBlockAnotherReport(t *testing.T) {
	ttlDay(t)
	cat := catalog.Load()
	h, small, big := catIDs(cat)
	p := cachePath(t)
	_, _ = discovery.MergeReported(p, h, []string{small}, t0, cat)
	r, _ := discovery.MergeReported(p, h, []string{big}, t0.Add(time.Minute), cat)
	if !r.Written {
		t.Fatalf("%+v", r)
	}
}

func TestMergeReported_ConcurrentWithMergeKeepsValidJSON(t *testing.T) {
	ttlDay(t)
	cat := catalog.Load()
	h, small, big := catIDs(cat)
	p := cachePath(t)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); _, _ = discovery.MergeReported(p, h, []string{small, big}, t0, cat) }()
		go func() {
			defer wg.Done()
			_ = discovery.Merge(p, map[string]discovery.Entry{"other": detected(small)})
		}()
	}
	wg.Wait()
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	f := discovery.Read(p)
	if len(f.Harnesses) == 0 || len(data) == 0 {
		t.Fatalf("invalid cache: %s", data)
	}
	if runtime.GOOS != "windows" {
		if st, _ := os.Stat(p); st.Mode().Perm() != 0o600 {
			t.Errorf("mode %v", st.Mode().Perm())
		}
	}
}

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
	if e.Source != "mcp" || !e.FetchedAt.Equal(t0) || len(e.Models) != 2 || len(e.IDs) == 0 {
		t.Fatalf("entry %+v", e)
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
	if r.Accepted != discovery.MaxReported || len(discovery.Read(p).Harnesses[h].Models) != 64 {
		t.Fatalf("result %+v", r)
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

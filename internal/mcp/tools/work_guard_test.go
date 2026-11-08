// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/workers"
)

func TestDelegateDirContainment(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "sub"), 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	cases := []struct {
		name, dir string
		ok        bool
	}{
		{"subdir", filepath.Join(root, "sub"), true},
		{"root itself", root, true},
		{"not yet existing child", filepath.Join(root, "new", "deep"), true},
		{"outside", outside, false},
		{"dotdot", filepath.Join(root, "sub", "..", ".."), false},
		{"escaping symlink", link, false},
		{"child of escaping symlink", filepath.Join(link, "x"), false},
		// Not filepath.Join: it would clean the ".." away before the check sees it.
		{"symlink then dotdot", link + string(filepath.Separator) + "..", false},
		{"inner dotdot", root + string(filepath.Separator) + "sub" + string(filepath.Separator) + "..", false},
	}
	for _, mode := range []string{"read", "write"} {
		for _, c := range cases {
			d, _ := delegateDeps(root, false, nil)
			raw, _ := json.Marshal(map[string]any{"tasks": []string{"x"}, "agent": "claude", "mode": mode, "dir": c.dir})
			_, err := d.delegate(context.Background(), raw)
			if c.ok && err != nil {
				t.Errorf("%s/%s: unexpected refusal: %v", mode, c.name, err)
			}
			if !c.ok && (err == nil || !strings.Contains(err.Error(), "fix:")) {
				t.Errorf("%s/%s: want refusal with a fix line, got %v", mode, c.name, err)
			}
		}
	}
}

func TestDelegateRefusesPastTheSessionCap(t *testing.T) {
	d, _ := delegateDeps(t.TempDir(), false, nil)
	d.Work = workers.New(&stubLauncher{}, 4, time.Now)
	d.Work.SetMaxJobs(32)
	one := json.RawMessage(`{"tasks":["x"],"agent":"claude","mode":"read"}`)
	for i := 0; i < 32; i++ {
		if _, err := d.delegate(context.Background(), one); err != nil {
			t.Fatalf("job %d: %v", i+1, err)
		}
	}
	_, err := d.delegate(context.Background(), one)
	if err == nil || !strings.Contains(err.Error(), RefusalCap) {
		t.Fatalf("33rd job must be refused with %q, got %v", RefusalCap, err)
	}
}

func TestDelegateQuotaStopLine(t *testing.T) {
	cases := []struct {
		name    string
		entries []StrainEntry
		line    func() float64
		refuse  bool
		want    string
	}{
		{"below the line", []StrainEntry{{Provider: "claude", UsedPct: 40, Headroom: "ok"}}, nil, false, ""},
		{"past the default line", []StrainEntry{{Provider: "claude", UsedPct: 95, Headroom: "critical"}}, nil, true, "claude quota is 95% used"},
		{"past a configured line", []StrainEntry{{Provider: "claude", UsedPct: 60, Headroom: "ok"}}, func() float64 { return 50 }, true, "stop line of 50%"},
		{"unknown stays allow", []StrainEntry{{Provider: "claude", UsedPct: 0, Headroom: "unknown"}}, nil, false, ""},
		{"rate limited", []StrainEntry{{Provider: "claude", UsedPct: 10, RateLimited: true, Headroom: "critical"}}, nil, true, "rate limited"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d, l := delegateDeps(t.TempDir(), false, nil)
			d.Strain = fakeStrain{entries: c.entries}
			d.QuotaStop = c.line
			_, err := d.delegate(context.Background(), json.RawMessage(`{"tasks":["a","b"],"agent":"claude","mode":"read"}`))
			time.Sleep(50 * time.Millisecond)
			if !c.refuse {
				if err != nil || len(l.specs) != 2 {
					t.Fatalf("err = %v, started %d, want 2", err, len(l.specs))
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), RefusalQuota) || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want code %q and %q", err, RefusalQuota, c.want)
			}
			if len(l.specs) != 0 {
				t.Fatalf("started %d workers past the stop line", len(l.specs))
			}
		})
	}
}

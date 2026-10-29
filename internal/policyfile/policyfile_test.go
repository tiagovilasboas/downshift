// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package policyfile

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

func write(t *testing.T, dir, body string) string {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(RelPath))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestNoFileIsNoOp(t *testing.T) {
	p, path, err := LoadNearest(t.TempDir())
	if p != nil || path != "" || err != nil {
		t.Fatalf("got %v %q %v", p, path, err)
	}
	if _, ok := p.Floor("COMPLEX", []string{"secret"}); ok {
		t.Fatal("nil policy must apply nothing")
	}
}

func TestEmptyStartDir(t *testing.T) {
	if _, ok, err := Find(""); ok || err != nil {
		t.Fatal("empty start dir must find nothing")
	}
}

func TestNearestFileWinsAndSearchIsBounded(t *testing.T) {
	root := t.TempDir()
	write(t, root, `{"version":1,"min_tier":"mid"}`)
	sub := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	p, path, err := LoadNearest(sub)
	if err != nil || p == nil || !strings.HasPrefix(path, root) {
		t.Fatalf("root policy not found from subdir: %v %q", err, path)
	}
	inner := filepath.Join(root, "a")
	write(t, inner, `{"version":1,"min_tier":"frontier"}`)
	p, path, _ = LoadNearest(sub)
	if tier, _ := p.Floor("SIMPLE", nil); tier != core.TierFrontier || !strings.HasPrefix(path, inner) {
		t.Fatalf("nearest file must win, got %v from %s", tier, path)
	}
	deep := root
	for i := 0; i <= MaxDepth+1; i++ {
		deep = filepath.Join(deep, "d")
	}
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	// root policy is more than MaxDepth levels above deep.
	other := t.TempDir()
	write(t, other, `{"version":1,"min_tier":"mid"}`)
	far := other
	for i := 0; i <= MaxDepth+1; i++ {
		far = filepath.Join(far, "d")
	}
	if err := os.MkdirAll(far, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := Find(far); ok {
		t.Fatal("search must stop after MaxDepth levels")
	}
}

func TestFloorTakesHighestApplicableRule(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, `{"version":1,"min_tier":"small","min_tier_by_class":{"MEDIUM":"mid","COMPLEX":"frontier"},"min_tier_by_risk":{"payment":"frontier"}}`)
	p, _, err := LoadNearest(dir)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		class string
		risks []string
		want  core.Tier
	}{
		{"TRIVIAL", nil, core.TierSmall},
		{"MEDIUM", nil, core.TierMid},
		{"COMPLEX", nil, core.TierFrontier},
		{"TRIVIAL", []string{"payment"}, core.TierFrontier},
		{"MEDIUM", []string{"secret"}, core.TierMid},
	}
	for _, c := range cases {
		got, ok := p.Floor(c.class, c.risks)
		if !ok || got != c.want {
			t.Errorf("Floor(%s,%v) = %v,%v want %v", c.class, c.risks, got, ok, c.want)
		}
	}
}

// The routing section shares the file with the tier floor. Its presence must
// not refuse the floor, and the floor must still apply.
func TestRoutingSectionDoesNotDisableFloor(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, `{"version":1,"min_tier":"mid","routing":{"rules":[]}}`)
	p, _, err := LoadNearest(dir)
	if err != nil {
		t.Fatalf("routing section refused the floor: %v", err)
	}
	if got, ok := p.Floor("SIMPLE", nil); !ok || got != core.TierMid {
		t.Fatalf("Floor = %v,%v; want mid,true", got, ok)
	}
}

func TestInvalidFilesAreRejectedWithPath(t *testing.T) {
	bad := map[string]string{
		"not json":      `nope`,
		"wrong version": `{"version":2}`,
		"unknown tier":  `{"version":1,"min_tier":"huge"}`,
		"unknown class": `{"version":1,"min_tier_by_class":{"EASY":"mid"}}`,
		"unknown risk":  `{"version":1,"min_tier_by_risk":{"typo":"mid"}}`,
		"unknown field": `{"version":1,"min_tir":"mid"}`,
		"class tier":    `{"version":1,"min_tier_by_class":{"MEDIUM":"huge"}}`,
	}
	for name, body := range bad {
		dir := t.TempDir()
		write(t, dir, body)
		p, path, err := LoadNearest(dir)
		if p != nil || err == nil || path == "" {
			t.Errorf("%s: want error with path, got %v %q %v", name, p, path, err)
		}
	}
}

func TestOversizedAndNonRegularFilesAreRejected(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, `{"version":1,"min_tier":"mid","pad":"`+strings.Repeat("x", maxBytes)+`"}`)
	if _, _, err := LoadNearest(dir); err == nil {
		t.Fatal("oversized file must be rejected")
	}
	d2 := t.TempDir()
	target := filepath.Join(d2, "real.json")
	if err := os.WriteFile(target, []byte(`{"version":1,"min_tier":"frontier"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(d2, filepath.FromSlash(RelPath))
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skip("symlinks unavailable")
	}
	if _, ok, _ := Find(d2); ok {
		t.Fatal("a symlinked policy must be ignored")
	}
}

func TestKnownRiskCategoriesMatchCoreTable(t *testing.T) {
	names := core.RiskCategoryNames()
	for _, want := range []string{"secret", "payment", "destructive"} {
		if !names[want] {
			t.Errorf("missing category %s", want)
		}
	}
}

func TestHookDoesNothingWithoutProjectDir(t *testing.T) {
	// A policy in the process directory must not leak into offline tools that
	// run Route without a hook invocation (benchmark, try).
	dir := t.TempDir()
	write(t, dir, `{"version":1,"min_tier":"frontier"}`)
	old, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Skip("cannot chdir")
	}
	defer os.Chdir(old)
	d := core.Route("rename the variable userId to accountId in auth.ts", "claude-code", "")
	if d.Guards.PolicyFloor || d.Tier != core.TierSmall {
		t.Fatalf("no project dir must mean no floor, got tier %v floor %v", d.Tier, d.Guards.PolicyFloor)
	}
}

// Find reports a failed lookup as an error, and only a missing entry as absent.
// Treating a permission failure as absent would route as if no file existed.
func TestFindTellsAbsentFromUnlookable(t *testing.T) {
	t.Run("no file is absent without error", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "missing", "start")
		if p, ok, err := Find(dir); ok || err != nil || p != "" {
			t.Fatalf("got %q %v %v, want absent with no error", p, ok, err)
		}
	})
	t.Run("nearest regular file is found", func(t *testing.T) {
		root := t.TempDir()
		want := write(t, root, `{"version":1}`)
		p, ok, err := Find(filepath.Join(root, "a"))
		if !ok || err != nil || p != want {
			t.Fatalf("got %q %v %v, want %q", p, ok, err, want)
		}
	})
	t.Run("unsearchable directory is a permission error", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("directory search permission is not enforced on Windows")
		}
		if os.Geteuid() == 0 {
			t.Skip("root bypasses directory search permission")
		}
		parent := t.TempDir()
		sub := filepath.Join(parent, "sub")
		if err := os.Mkdir(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(parent, 0o600); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(parent, 0o755) })
		p, ok, err := Find(sub)
		if ok || p != "" || !errors.Is(err, fs.ErrPermission) {
			t.Fatalf("got %q %v %v, want a permission error and no file", p, ok, err)
		}
	})
}

// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/testenv"
)

const selfStamp = "2026-10-28T06:00:00Z"

// scaleBinaryProbe stretches the --version probe each fake copy answers, so a
// loaded machine starting /bin/sh slowly does not drop a copy from the list.
func scaleBinaryProbe(t *testing.T) {
	t.Helper()
	old := binaryProbeTimeout
	// A guard only: the fake copies answer at once, and a loaded machine can
	// take longer than the production 3 s just to start a shell.
	binaryProbeTimeout = testenv.Scaled(15 * time.Second)
	t.Cleanup(func() { binaryProbeTimeout = old })
}

func TestPlanPrune(t *testing.T) {
	older := binaryCopy{path: "/bin/old", built: "2026-10-20T10:00:00Z"}
	same := binaryCopy{path: "/bin/same", built: selfStamp}
	unstamped := binaryCopy{path: "/bin/legacy", built: ""}
	newer := binaryCopy{path: "/bin/new", built: "2026-10-29T10:00:00Z"}
	cases := []struct {
		name      string
		self      string
		copies    []binaryCopy
		configs   [][]byte
		remove    []string
		newer     string
		unstamped bool
	}{
		{name: "older copy is removed", self: selfStamp, copies: []binaryCopy{older}, remove: []string{"/bin/old"}},
		{name: "same build is removed", self: selfStamp, copies: []binaryCopy{same}, remove: []string{"/bin/same"}},
		{name: "copy with no stamp is removed", self: selfStamp, copies: []binaryCopy{unstamped}, remove: []string{"/bin/legacy"}},
		{name: "newer copy stops the step", self: selfStamp, copies: []binaryCopy{older, newer}, newer: "/bin/new"},
		{name: "unstamped build removes nothing", self: "", copies: []binaryCopy{older}, unstamped: true},
		{name: "copy named by a config is kept",
			self: selfStamp, copies: []binaryCopy{older},
			configs: [][]byte{[]byte(`{"command":"/bin/old quota"}`)}},
		{name: "no copies, nothing to do", self: selfStamp},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := planPrune(c.copies, c.self, c.configs)
			if p.unstamped != c.unstamped {
				t.Errorf("unstamped = %v, want %v", p.unstamped, c.unstamped)
			}
			if p.newer != c.newer {
				t.Errorf("newer = %q, want %q", p.newer, c.newer)
			}
			var got []string
			for _, r := range p.remove {
				got = append(got, r.path)
			}
			if strings.Join(got, ",") != strings.Join(c.remove, ",") {
				t.Errorf("remove = %v, want %v", got, c.remove)
			}
		})
	}
}

// fakeDownshift writes an executable that answers --version as downshift
// does, with the given build stamp; an empty stamp writes the older line.
func fakeDownshift(t *testing.T, dir, built string) string {
	t.Helper()
	line := "downshift v0.2.0 (commit abc1234, built " + built + ")"
	if built == "" {
		line = "downshift v0.1.0"
	}
	p := filepath.Join(dir, BinaryName())
	script := "#!/bin/sh\necho '" + line + "'\n"
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestFindBinaryCopies(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake executables are shell scripts")
	}
	scaleBinaryProbe(t)
	self := t.TempDir()
	selfBin := fakeDownshift(t, self, selfStamp)
	old := t.TempDir()
	oldBin := fakeDownshift(t, old, "2026-10-20T10:00:00Z")
	foreign := t.TempDir()
	if err := os.WriteFile(filepath.Join(foreign, BinaryName()), []byte("#!/bin/sh\necho hello\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	empty := t.TempDir()
	links := t.TempDir()
	if err := os.Symlink(oldBin, filepath.Join(links, BinaryName())); err != nil {
		t.Fatal(err)
	}

	got := findBinaryCopies([]string{self, old, foreign, empty, links}, selfBin)
	if len(got) != 1 || got[0].path != oldBin || got[0].built != "2026-10-20T10:00:00Z" {
		t.Fatalf("findBinaryCopies = %+v, want only %s", got, oldBin)
	}
}

func TestPruneBinariesPreviewThenRemove(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake executables are shell scripts")
	}
	scaleBinaryProbe(t)
	self := t.TempDir()
	selfBin := fakeDownshift(t, self, selfStamp)
	old := t.TempDir()
	oldBin := fakeDownshift(t, old, "2026-10-20T10:00:00Z")
	home := t.TempDir()
	base := Options{Home: home, Binary: selfBin, BinDirs: []string{self, old}, BuildDate: selfStamp}

	var out, errw bytes.Buffer
	preview := base
	preview.DryRun, preview.Out, preview.Err = true, &out, &errw
	if pruneBinaries(preview, nil) {
		t.Fatal("dry run reported a failure")
	}
	if _, err := os.Stat(oldBin); err != nil {
		t.Fatalf("dry run removed the old copy: %v", err)
	}
	if !strings.Contains(out.String(), "would be removed") {
		t.Errorf("dry run did not preview the removal:\n%s", out.String())
	}

	out.Reset()
	apply := base
	apply.Yes, apply.Out, apply.Err, apply.In = true, &out, &errw, strings.NewReader("")
	if pruneBinaries(apply, nil) {
		t.Fatalf("removal failed: %s", errw.String())
	}
	if _, err := os.Stat(oldBin); !os.IsNotExist(err) {
		t.Errorf("old copy still exists after --yes: %v", err)
	}
	if _, err := os.Stat(selfBin); err != nil {
		t.Errorf("the running binary was removed: %v", err)
	}
}

func TestPruneBinariesWithoutYesKeepsCopy(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake executables are shell scripts")
	}
	self := t.TempDir()
	selfBin := fakeDownshift(t, self, selfStamp)
	old := t.TempDir()
	oldBin := fakeDownshift(t, old, "2026-10-20T10:00:00Z")
	var out bytes.Buffer
	o := Options{Home: t.TempDir(), Binary: selfBin, BinDirs: []string{old}, BuildDate: selfStamp,
		In: strings.NewReader("n\n"), Out: &out, Err: &out}
	if pruneBinaries(o, nil) {
		t.Fatal("a declined question must not count as a failure")
	}
	if _, err := os.Stat(oldBin); err != nil {
		t.Fatalf("declined question removed the copy: %v", err)
	}
}

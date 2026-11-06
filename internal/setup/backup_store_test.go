// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const bridgeDir = ".config/opencode/plugins/downshift"

// leftovers lists every backup or marker found under dir, whichever layout.
func leftovers(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.Contains(d.Name(), BackupInfix) {
			out = append(out, p)
		}
		return nil
	})
	return out
}

func stateMarkers(t *testing.T, e *setupEnv) []string {
	t.Helper()
	var out []string
	root := e.path(".downshift/backups")
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			out = append(out, p)
		}
		return nil
	})
	return out
}

func TestWriteKeepsMarkersOutOfTheHarnessPluginFolder(t *testing.T) {
	e := newSetupEnv(t, ".config/opencode")
	if code, out, errOut := e.run("--yes", "--mcp", "opencode"); code != 0 {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	entries, _ := os.ReadDir(e.path(bridgeDir))
	if len(entries) == 0 {
		t.Fatal("the bridge was not written")
	}
	for _, en := range entries {
		if strings.Contains(en.Name(), BackupInfix) {
			t.Errorf("marker %s landed in the plugin folder", en.Name())
		}
	}
	if left := leftovers(t, e.path(".config")); len(left) != 0 {
		t.Errorf("markers left in the harness folder: %v", left)
	}
	if got := stateMarkers(t, e); len(got) == 0 {
		t.Error("no marker in the state dir")
	}
}

func TestUndoRemovesNewStyleMarkersAndFolders(t *testing.T) {
	e := newSetupEnv(t, ".config/opencode")
	e.run("--yes", "--mcp", "opencode")
	if code, out, errOut := e.run("--undo", "--yes", "--mcp", "opencode"); code != 0 {
		t.Fatalf("undo exit %d\n%s\n%s", code, out, errOut)
	}
	if _, err := os.Stat(e.path(bridgeDir)); err == nil {
		t.Error("the bridge folder survived the undo")
	}
	if _, err := os.Stat(e.path(".config/opencode/plugins")); err == nil {
		t.Error("the plugins folder setup created survived the undo")
	}
}

func TestUndoStillHonoursOldStyleMarkers(t *testing.T) {
	e := newSetupEnv(t, ".config/opencode")
	e.run("--yes", "--mcp", "opencode")
	// Rebuild the layout of an older setup: markers next to the files.
	if err := os.RemoveAll(e.path(".downshift/backups")); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(e.path(bridgeDir))
	for _, en := range entries {
		old := e.path(bridgeDir + "/" + en.Name() + BackupInfix + "20260101T000000.000000.new")
		if err := os.WriteFile(old, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if code, out, errOut := e.run("--undo", "--yes", "--mcp", "opencode"); code != 0 {
		t.Fatalf("undo exit %d\n%s\n%s", code, out, errOut)
	}
	if _, err := os.Stat(e.path(bridgeDir)); err == nil {
		t.Error("old-style markers: the bridge folder survived the undo")
	}
}

func TestLoadedFileBackupGoesToTheStateDirAndUndoRestoresIt(t *testing.T) {
	e := newSetupEnv(t, ".config/opencode")
	e.run("--yes", "--mcp", "opencode")
	index := bridgeDir + "/index.ts"
	edited := e.read(index) + "\n// edited\n"
	e.write(index, edited)
	e.run("--yes", "--mcp", "opencode") // refreshes the owned file, backing up the edit
	if left := leftovers(t, e.path(".config")); len(left) != 0 {
		t.Fatalf("backups left in the harness folder: %v", left)
	}
	if strings.Contains(e.read(index), "// edited") {
		t.Fatal("the owned bridge was not refreshed")
	}
	if code, out, errOut := e.run("--undo", "--yes", "--mcp", "opencode"); code != 0 {
		t.Fatalf("undo exit %d\n%s\n%s", code, out, errOut)
	}
	if got := e.read(index); got != edited {
		t.Errorf("undo did not restore the backed-up file")
	}
}

func TestPruneKeepsOnlyTheNewestBackupInTheStateDir(t *testing.T) {
	e := newSetupEnv(t, ".config/opencode")
	e.run("--yes", "--mcp", "opencode")
	index := bridgeDir + "/index.ts"
	for i := 0; i < 3; i++ {
		e.write(index, e.read(index)+"\n// edit\n")
		e.run("--yes", "--mcp", "opencode")
	}
	n := 0
	for _, p := range stateMarkers(t, e) {
		if strings.HasSuffix(p, "plugins"+string(filepath.Separator)+"downshift"+string(filepath.Separator)+"index.ts") {
			n++
		}
	}
	if n != 1 {
		t.Errorf("want one backup of index.ts kept, got %d", n)
	}
}

var fileLine = regexp.MustCompile(`^\S+: (~/\S+) (would change|will change|written)`)

func TestDryRunAndApplyReportTheSameFiles(t *testing.T) {
	e := newSetupEnv(t, ".config/opencode")
	_, dry, _ := e.run("--dry-run", "--mcp", "opencode")
	_, applied, _ := e.run("--yes", "--mcp", "opencode")
	files := func(s, verb string) map[string]bool {
		got := map[string]bool{}
		for _, l := range strings.Split(s, "\n") {
			if m := fileLine.FindStringSubmatch(l); m != nil && strings.HasPrefix(m[2], verb) {
				got[m[1]] = true
			}
		}
		return got
	}
	previewed, written := files(dry, "would"), files(applied, "written")
	if len(previewed) == 0 || len(previewed) != len(written) {
		t.Fatalf("dry run named %v, apply wrote %v", previewed, written)
	}
	for f := range previewed {
		if !written[f] {
			t.Errorf("%s previewed but not reported written", f)
		}
	}
}

// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package archtest

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// maxFileLines is the common-gate limit for a production Go file.
const maxFileLines = 300

// oversizedBaseline freezes production files that are over the limit. It
// listed 20 files when this sensor was added (2026-10-30); all of them were
// split by 2026-11-02, so it is empty and every production file must stay
// within maxFileLines. Do not add an entry: split the file instead.
var oversizedBaseline = map[string]int{}

// productionGoLines returns the line count of every production .go file under
// the repository, keyed by its slash path relative to the root.
func productionGoLines(t *testing.T) map[string]int {
	t.Helper()
	out := map[string]int{}
	err := filepath.WalkDir(repoRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if path != repoRoot && (strings.HasPrefix(name, ".") || name == "node_modules" || name == "testdata" || name == "vendor") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(repoRoot, path)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = bytes.Count(raw, []byte("\n"))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// TestProductionFileSize is the computational sensor for the "no file over
// 300 lines" rule: a new file over the limit fails, a baseline file that grows
// fails, and a baseline entry that no longer applies fails (delete it).
func TestProductionFileSize(t *testing.T) {
	lines := productionGoLines(t)
	for path, n := range lines {
		limit, frozen := oversizedBaseline[path]
		switch {
		case frozen && n > limit:
			t.Errorf("%s has %d lines, above its frozen %d; split it instead of growing it", path, n, limit)
		case !frozen && n > maxFileLines:
			t.Errorf("%s has %d lines, over the %d-line limit; split it by what each part does", path, n, maxFileLines)
		}
	}
	for path, limit := range oversizedBaseline {
		n, ok := lines[path]
		switch {
		case !ok:
			t.Errorf("baseline entry %s (%d) names a file that no longer exists: remove it", path, limit)
		case n <= maxFileLines:
			t.Errorf("%s is now %d lines, within the limit: remove its baseline entry", path, n)
		case n < limit:
			t.Errorf("%s shrank to %d lines: lower its baseline entry from %d", path, n, limit)
		}
	}
}

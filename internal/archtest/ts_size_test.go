// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package archtest

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// maxTSLines is the common-gate limit for a TypeScript file.
const maxTSLines = 300

// tsSizeBaseline freezes TypeScript files over the limit when this sensor was
// added, each with its line count as a ceiling. Do not add an entry: split the
// file instead.
var tsSizeBaseline = map[string]int{
	"hooks/opencode/index.ts":                               329,
	"mods/downshift-agents/hooks/shared/ui/svg/svg.test.ts": 310,
}

// TestTypeScriptFileSize is the computational sensor for "no file over 300
// lines" on the TypeScript under mods/ and hooks/ (node_modules and .d.ts
// files are skipped).
func TestTypeScriptFileSize(t *testing.T) {
	lines := map[string]int{}
	for _, p := range walkFiles(t, ".ts") {
		underScope := strings.HasPrefix(p, "mods/") || strings.HasPrefix(p, "hooks/")
		if !underScope || strings.HasSuffix(p, ".d.ts") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(p)))
		if err != nil {
			t.Fatal(err)
		}
		lines[p] = bytes.Count(raw, []byte("\n"))
	}
	for path, n := range lines {
		ceiling, frozen := tsSizeBaseline[path]
		switch {
		case frozen && n > ceiling:
			t.Errorf("%s has %d lines, above its frozen %d. Rule: at most %d lines per file. Fix: split it by what each part does.", path, n, ceiling, maxTSLines)
		case frozen && n <= maxTSLines:
			t.Errorf("%s is now %d lines, within the limit: remove its tsSizeBaseline entry", path, n)
		case frozen && n < ceiling:
			t.Errorf("%s shrank to %d lines: lower its tsSizeBaseline entry from %d", path, n, ceiling)
		case !frozen && n > maxTSLines:
			t.Errorf("%s has %d lines, over the %d-line limit. Rule: split files by responsibility. Fix: move one responsibility to its own file; do not add to tsSizeBaseline.", path, n, maxTSLines)
		}
	}
	for path, ceiling := range tsSizeBaseline {
		if _, ok := lines[path]; !ok {
			t.Errorf("tsSizeBaseline entry %s (%d) names a file that no longer exists: remove it", path, ceiling)
		}
	}
}

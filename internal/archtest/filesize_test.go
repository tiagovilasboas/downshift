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

// oversizedBaseline freezes the production files that were already over the
// limit when this sensor was added (2026-10-30). Each may shrink, never grow;
// a file not listed may not cross the limit. When a listed file drops to the
// limit or below, delete its entry. Keep this list shrinking, never growing.
var oversizedBaseline = map[string]int{
	"cmd/downshift/context.go":                   464,
	"cmd/downshift/dci.go":                       590,
	"cmd/downshift/doctor.go":                    436,
	"cmd/downshift/doctor_hooks.go":              358,
	"cmd/downshift/hookrun.go":                   393,
	"cmd/downshift/setup.go":                     388,
	"cmd/downshift/setup_merge.go":               309,
	"cmd/dsmon/main.go":                          574,
	"internal/adapters/claudecode/claudecode.go": 368,
	"internal/adapters/cursor/cursor.go":         396,
	"internal/adapters/opencode/port.go":         313,
	"internal/benchmark/benchmark.go":            594,
	"internal/catalog/catalog.go":                869,
	"internal/childend/record.go":                555,
	"internal/classify/classifier.go":            515,
	"internal/classify/risk.go":                  319,
	"internal/codeintel/parse.go":                318,
	"internal/codeintel/parse_lang.go":           1244,
	"internal/codeintel/rank.go":                 381,
	"internal/codeintel/security.go":             682,
	"internal/codeintel/telemetry.go":            325,
	"internal/compressor/compressor.go":          1099,
	"internal/compressor/gh_run_log.go":          305,
	"internal/compressor/git_diff.go":            339,
	"internal/core/policy.go":                    307,
	"internal/core/session_select.go":            337,
	"internal/discovery/sources.go":              342,
	"internal/execwrap/policy_tooling.go":        851,
	"internal/execwrap/run.go":                   460,
	"internal/quota/native.go":                   368,
	"internal/telemetry/aggregate.go":            302,
	"internal/telemetry/report.go":               305,
	"internal/telemetry/telemetry.go":            840,
	"internal/telemetry/tokens_rerouted.go":      322,
}

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

// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/tiagovilasboas/downshift-ai/internal/setup"
	"github.com/tiagovilasboas/downshift-ai/internal/setup/configfile"
)

// The generated-file check for `doctor`. A harness whose glue is a file setup
// writes whole (setup.Target.Files, the OpenCode bridge) has no hook file for
// checkHarness to read. This check compares each file with the bytes setup
// would write now: a missing file, a foreign file and a stale file each get
// their own row and fix. The expected bytes come from the same embedded source
// setup uses, so a binary upgrade shows as stale until setup runs again.

// checkGeneratedFiles reports the setup.Target.Files of one harness and returns
// whether the harness is detected (its config directory exists). A harness
// without Files returns false and reports nothing.
func checkGeneratedFiles(add func(id, status, detail, fix string), home, harness string) bool {
	t, ok := generatedFilesTarget(harness)
	if !ok || !fileExists(filepath.Join(home, filepath.FromSlash(t.ConfigDir))) {
		return false
	}
	// One row for the bridge: the worst state of its files, the first on a tie.
	var worst *doctorRow
	for _, f := range t.Files {
		checkGeneratedFile(func(id, status, detail, fix string) {
			if worst == nil || doctorSeverity(status) > doctorSeverity(worst.status) {
				worst = &doctorRow{id, status, detail, fix}
			}
		}, home, f)
	}
	if worst != nil {
		add(worst.id, worst.status, worst.detail, worst.fix)
	}
	return true
}

type doctorRow struct{ id, status, detail, fix string }

// doctorSeverity orders the statuses: ok, warn, fail.
func doctorSeverity(status string) int {
	switch status {
	case doctorFail:
		return 2
	case doctorWarn:
		return 1
	}
	return 0
}

func generatedFilesTarget(harness string) (setup.Target, bool) {
	for _, t := range setup.Targets {
		if t.Name == harness && len(t.Files) > 0 {
			return t, true
		}
	}
	return setup.Target{}, false
}

func checkGeneratedFile(add func(id, status, detail, fix string), home string, f setup.HarnessFile) {
	const id = "bridge"
	path := filepath.Join(home, filepath.FromSlash(f.Dest))
	shown := setup.Display(home, path)
	want, err := fs.ReadFile(f.FS, f.Source)
	if err != nil {
		add(id, doctorFail, fmt.Sprintf("%s: this binary carries no copy of the file (%v)", shown, err), "")
		return
	}
	cur, err := os.ReadFile(path)
	switch {
	case os.IsNotExist(err):
		add(id, doctorWarn, shown+" is not installed", "downshift setup --yes")
	case err != nil:
		add(id, doctorFail, fmt.Sprintf("%s cannot be read: %v", shown, err), "")
	case !configfile.Owns(cur):
		add(id, doctorWarn, shown+" was not written by downshift setup, so setup will not replace it",
			"move "+shown+" aside, then downshift setup --yes")
	case !bytes.Equal(cur, want):
		add(id, doctorWarn, shown+" is older than this binary", "downshift setup --yes")
	default:
		add(id, doctorOK, shown+" is current", "")
	}
}

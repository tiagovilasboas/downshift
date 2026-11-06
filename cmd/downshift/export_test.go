// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/operatorexport"
	"github.com/tiagovilasboas/downshift/internal/testenv"
)

func isolateExport(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	testenv.SetHome(t, home)
	t.Setenv("DOWNSHIFT_STATE_DIR", filepath.Join(home, ".downshift"))
}

func TestRunExportRefusesBadFlags(t *testing.T) {
	isolateExport(t)
	for _, args := range [][]string{{"--days=x"}, {"--days=-1"}, {"--json"}} {
		var out, errOut bytes.Buffer
		if code := runExport(args, &out, &errOut); code != 2 {
			t.Errorf("runExport(%v) = %d, want 2", args, code)
		}
		if out.Len() != 0 {
			t.Errorf("runExport(%v) printed to stdout on a usage error: %q", args, out.String())
		}
	}
}

// TestRunExportPrintsCheckedJSON runs the real command on an empty home: it
// must exit 0 and print JSON that the allowlist passes.
func TestRunExportPrintsCheckedJSON(t *testing.T) {
	isolateExport(t)
	var out, errOut bytes.Buffer
	if code := runExport([]string{"--days=7"}, &out, &errOut); code != 0 {
		t.Fatalf("runExport = %d, stderr %q", code, errOut.String())
	}
	var e operatorexport.Export
	if err := json.Unmarshal(out.Bytes(), &e); err != nil {
		t.Fatalf("stdout is not an export: %v\n%s", err, out.String())
	}
	if e.WindowDays != 7 || e.Note != operatorexport.Note {
		t.Errorf("window_days=%d note=%q", e.WindowDays, e.Note)
	}
	if err := operatorexport.CheckJSON(out.Bytes()); err != nil {
		t.Errorf("printed export fails the allowlist: %v", err)
	}
}

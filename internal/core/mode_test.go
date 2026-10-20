// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/paths"
)

func TestModeFileWinsOverEnvAndDefaultsToPerformance(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(paths.EnvStateDir, dir)
	noEnv := func(string) string { return "" }
	economyEnv := func(k string) string {
		if k == ModeEnv {
			return ModeEconomy
		}
		return ""
	}
	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, ModeFile), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if economyFrom(noEnv) {
		t.Fatal("no file and no env must be performance")
	}
	if !economyFrom(economyEnv) {
		t.Fatal("env alone must select economy when there is no file")
	}

	write(`{"mode":"economy"}`)
	if !economyFrom(noEnv) {
		t.Error("file economy must select economy with no env")
	}
	write(`{"mode":"performance"}`)
	if economyFrom(economyEnv) {
		t.Error("file performance must win over env economy")
	}

	write(`not json`)
	if !economyFrom(economyEnv) {
		t.Error("an invalid file must fall back to the env")
	}
	write(`{"mode":"turbo"}`)
	if economyFrom(noEnv) {
		t.Error("an unknown mode must fall back to performance")
	}
}

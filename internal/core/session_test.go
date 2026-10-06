// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/tiagovilasboas/harness-downshift/internal/core"
)

func TestLoadSessionFileForID_PrefersSessionOverHarness(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session-models.json")
	data := []byte(`{"codex":["gpt-5.6-sol"],"sessions":{"codex":{"session-1":["gpt-6-luna","gpt-6-sol"]}}}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	got := core.LoadSessionFileForID("codex", "session-1", path)
	if !got.Known || !reflect.DeepEqual(got.IDs, []string{"gpt-6-luna", "gpt-6-sol"}) {
		t.Fatalf("session-specific list = %#v, want session-1 allowlist", got)
	}
}

func TestLoadSessionFileForID_FallsBackToHarness(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session-models.json")
	data := []byte(`{"codex":["gpt-6-luna","gpt-6-sol"],"sessions":{"codex":{"other-session":["gpt-5.6-sol"]}}}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	got := core.LoadSessionFileForID("codex", "session-1", path)
	if !got.Known || !reflect.DeepEqual(got.IDs, []string{"gpt-6-luna", "gpt-6-sol"}) {
		t.Fatalf("harness fallback = %#v, want Codex allowlist", got)
	}
}

// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestDocsManifestGolden pins the manifest bytes. Regenerate with:
//
//	go test ./cmd/downshift -run TestDocsManifestGolden -update
func TestDocsManifestGolden(t *testing.T) {
	var buf bytes.Buffer
	if rc := runDocsManifest(&buf); rc != 0 {
		t.Fatalf("exit %d", rc)
	}
	if !json.Valid(buf.Bytes()) {
		t.Fatal("manifest is not valid JSON")
	}
	var again bytes.Buffer
	runDocsManifest(&again)
	if !bytes.Equal(buf.Bytes(), again.Bytes()) {
		t.Fatal("manifest is not deterministic")
	}
	path := filepath.Join("testdata", "docs-manifest.golden")
	if *updateGoldens {
		if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing golden %s (run with -update): %v", path, err)
	}
	if string(want) != buf.String() {
		t.Errorf("docs-manifest changed; review it and rerun with -update\n%s", buf.String())
	}
}

func TestDocsManifestListsRealRegistries(t *testing.T) {
	m := buildDocsManifest()
	for _, tool := range []string{"providers", "apply", "retrieve", "delegate"} {
		found := false
		for _, n := range m.MCPTools {
			found = found || n == tool
		}
		if !found {
			t.Errorf("MCP tool %q missing from the manifest", tool)
		}
	}
	if len(m.Harnesses) == 0 || m.Endpoints["9router"] == "" {
		t.Errorf("harnesses or endpoints empty: %+v", m)
	}
}

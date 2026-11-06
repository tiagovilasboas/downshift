// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDoctorHarnessFilter(t *testing.T) {
	f := newDoctorFixture(t)
	if err := os.MkdirAll(filepath.Join(f.home, ".codex"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.home, ".codex", "config.toml"), []byte("# downshift:mcp:begin\n[mcp_servers.downshift]\ncommand = \"downshift\"\n# downshift:mcp:end\n"), 0600); err != nil {
		t.Fatal(err)
	}
	f.env.Harness = "codex"
	rep := buildDoctorReport(f.env)
	if c, ok := findCheck(rep, "codex", "codex mcp registered"); !ok || c.Status != doctorOK {
		t.Fatalf("missing codex MCP registration: %+v", c)
	}
	data, err := json.Marshal(rep)
	if err != nil {
		t.Fatal(err)
	}
	var decoded doctorReport
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, c := range decoded.Checks {
		if c.Harness != "" && c.Harness != "codex" {
			t.Fatalf("unexpected harness %q", c.Harness)
		}
	}
	for _, h := range decoded.NotDetected {
		if h != "codex" {
			t.Fatalf("unexpected skipped harness %q", h)
		}
	}
	var out, errOut bytes.Buffer
	if code := runDoctor([]string{"--harness", "unknown-harness"}, &out, &errOut, nil); code != 2 || !strings.Contains(errOut.String(), "unknown harness") {
		t.Fatalf("code %d, error %q", code, errOut.String())
	}
}

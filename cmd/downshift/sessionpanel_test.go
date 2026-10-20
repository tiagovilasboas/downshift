// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/paths"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// A wrapped command carries the hashed session, the runner logs it, and the
// session panel reports only that session's Compactor bytes.
func TestPanel_CompactorIsPerSession(t *testing.T) {
	t.Setenv(paths.EnvStateDir, t.TempDir())
	bin := t.TempDir()
	var suite strings.Builder
	for i := 0; i < 8; i++ {
		fmt.Fprintf(&suite, "ok  \tsecret/pkg/p%d\t0.1s\n", i)
	}
	if err := os.WriteFile(filepath.Join(bin, "suite.txt"), []byte(suite.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\ncat \"$(dirname \"$0\")/suite.txt\"\n"
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	mine, other := telemetry.HashSessionID("s-mine"), telemetry.HashSessionID("s-other")
	for _, h := range []string{mine, mine, other} {
		var out, errb bytes.Buffer
		if rc := runExec([]string{"--session=" + h, "--", "go", "test", "./secret/..."}, strings.NewReader(""), &out, &errb); rc != 0 {
			t.Fatalf("rc = %d, stderr %q", rc, errb.String())
		}
	}
	// A malformed session is ignored, not logged.
	var out, errb bytes.Buffer
	runExec([]string{"--session=../x", "--", "go", "test", "./secret/..."}, strings.NewReader(""), &out, &errb)

	raw, err := json.Marshal(panelWithCompactor(telemetry.PanelView{Session: mine}, mine))
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Compactor *compactorPanel `json:"compactor"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Compactor == nil || got.Compactor.Commands != 2 || got.Compactor.Compacted != 2 || got.Compactor.SavedBytes <= 0 {
		t.Fatalf("compactor block = %+v, want 2 compacted commands of this session", got.Compactor)
	}
	if strings.Contains(string(raw), "s-mine") {
		t.Fatalf("the raw session id leaked into the panel: %s", raw)
	}

	empty, _ := json.Marshal(panelWithCompactor(telemetry.PanelView{}, telemetry.HashSessionID("none")))
	if strings.Contains(string(empty), "compactor") {
		t.Fatalf("a session with no wrapped commands must carry no compactor block: %s", empty)
	}
}

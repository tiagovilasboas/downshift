// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/ccr"
	"github.com/tiagovilasboas/downshift/internal/execwrap"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// A compacted run whose kept output the agent then fetches back with
// `downshift retrieve` reports no saving in the report (text and JSON) and the
// session panel, and the fetched bytes show on their own line.
func TestReportAndPanel_FullyRetrievedRunSavesNothing(t *testing.T) {
	endEnv(t)
	hash := telemetry.HashSessionID("s-retrieved")
	raw := strings.Repeat("ok  \tpkg/x\t0.01s\n", 1200)
	store, err := ccr.Default()
	if err != nil {
		t.Fatal(err)
	}
	id, err := store.Put([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	path, err := execwrap.EventsPath()
	if err != nil {
		t.Fatal(err)
	}
	ev := execwrap.EventFrom(execwrap.Result{
		Family: execwrap.FamilyGoTest, RawBytes: int64(len(raw)), EmittedBytes: 300,
		ExitClass: execwrap.ExitClassZero, Compacted: true, KeptID: id,
	}, time.Now())
	ev.Session, ev.Harness = hash, "claude-code"
	if err := execwrap.Append(path, ev); err != nil {
		t.Fatal(err)
	}

	type compactor struct {
		SavedBytes     int64 `json:"saved_bytes"`
		RetrievedBytes int64 `json:"retrieved_bytes"`
		Tokens         int64 `json:"tokens"`
	}
	readReport := func() (compactor, string) {
		var out, errOut bytes.Buffer
		if rc := runReport([]string{"--json"}, &out, &errOut); rc != 0 {
			t.Fatalf("rc=%d %s", rc, errOut.String())
		}
		var r struct {
			Numbers struct {
				Compactor compactor `json:"compactor"`
				Display   struct {
					Retrieved string `json:"retrieved"`
				} `json:"display"`
			} `json:"numbers"`
		}
		if err := json.Unmarshal(out.Bytes(), &r); err != nil {
			t.Fatal(err)
		}
		return r.Numbers.Compactor, r.Numbers.Display.Retrieved
	}
	// The default (markdown) and --text renderers both carry the line.
	reportText := func() string {
		var all strings.Builder
		for _, args := range [][]string{nil, {"--text"}} {
			var out, errOut bytes.Buffer
			if rc := runReport(args, &out, &errOut); rc != 0 {
				t.Fatalf("rc=%d %s", rc, errOut.String())
			}
			all.WriteString(out.String())
		}
		return all.String()
	}

	before, line := readReport()
	if before.SavedBytes != int64(len(raw))-300 || before.RetrievedBytes != 0 || line != "" {
		t.Fatalf("before any retrieval: %+v %q", before, line)
	}
	if strings.Contains(reportText(), "Fetched back with retrieve") {
		t.Fatal("the retrieved line must not show when nothing was retrieved")
	}

	if rc := retrieveRun(id, ccr.Query{}, &bytes.Buffer{}, &bytes.Buffer{}); rc != 0 {
		t.Fatalf("retrieve rc=%d", rc)
	}

	after, line := readReport()
	if after.SavedBytes != 0 || after.Tokens != 0 || after.RetrievedBytes != int64(len(raw)) {
		t.Fatalf("report after retrieving it all: %+v", after)
	}
	if !strings.Contains(line, strconv.Itoa(len(raw))+" bytes") {
		t.Errorf("retrieved line = %q", line)
	}
	want := "Fetched back with retrieve: " + strconv.Itoa(len(raw)) + " bytes"
	if text := reportText(); strings.Count(text, want) != 2 {
		t.Errorf("want the retrieved line once in the markdown and once in the text report:\n%s", text)
	}

	var out, errOut bytes.Buffer
	if rc := writePanel(nil, hash, panelOptions{}, &out, &errOut); rc != 0 {
		t.Fatalf("panel rc=%d %s", rc, errOut.String())
	}
	var panel struct {
		Compactor *compactor `json:"compactor"`
	}
	if err := json.Unmarshal(out.Bytes(), &panel); err != nil {
		t.Fatal(err)
	}
	if panel.Compactor == nil || panel.Compactor.SavedBytes != 0 || panel.Compactor.RetrievedBytes != int64(len(raw)) {
		t.Fatalf("panel compactor = %+v", panel.Compactor)
	}
}

// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/execwrap"
)

// The text report ends with the one final figure and shows no separate
// Compactor byte or token lines; --json keeps the parts and adds the total.
func TestReport_EndsWithTokensSavedEstimate(t *testing.T) {
	endEnv(t)
	var out, errOut bytes.Buffer
	if rc := runReport(nil, &out, &errOut); rc != 0 {
		t.Fatalf("rc=%d %s", rc, errOut.String())
	}
	text := strings.TrimRight(out.String(), "\n")
	lines := strings.Split(text, "\n")
	if last := lines[len(lines)-1]; last != "Tokens saved (estimate): 0 (last 30 days: routing + compactor)" {
		t.Fatalf("last line = %q, want the final estimate with its window", last)
	}
	for _, gone := range []string{"Bytes:", "Tokens saved: ~"} {
		if strings.Contains(text, gone) {
			t.Errorf("text report still has %q:\n%s", gone, text)
		}
	}

	out.Reset()
	runReport([]string{"--json"}, &out, &errOut)
	var rep map[string]json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"tokens_saved", "compactor_saved_bytes", "compactor_tokens_estimate", "tokens_saved_estimate"} {
		if _, ok := rep[key]; !ok {
			t.Errorf("--json lacks %q", key)
		}
	}
	if _, ok := rep["numbers"]; !ok {
		t.Errorf("--json lacks numbers")
	}
}

// The Compactor is windowed like routing: an exec run older than --days is
// not in the total, and --days=0 (all time) takes it back.
func TestReport_CompactorUsesTheReportWindow(t *testing.T) {
	endEnv(t)
	path, err := execwrap.EventsPath()
	if err != nil {
		t.Fatal(err)
	}
	old := execwrap.Event{TS: time.Now().UTC().Add(-40 * 24 * time.Hour).Format(time.RFC3339), RawBytes: 4000, Compacted: true}
	recent := execwrap.Event{TS: time.Now().UTC().Add(-time.Hour).Format(time.RFC3339), RawBytes: 400, EmittedBytes: 100, Compacted: true}
	for _, ev := range []execwrap.Event{old, recent} {
		if err := execwrap.Append(path, ev); err != nil {
			t.Fatal(err)
		}
	}

	type report struct {
		CompactorSavedBytes int64 `json:"compactor_saved_bytes"`
		TokensSavedEstimate int64 `json:"tokens_saved_estimate"`
		Numbers             struct {
			Compactor struct {
				SavedBytes int64 `json:"saved_bytes"`
				Tokens     int64 `json:"tokens"`
			} `json:"compactor"`
		} `json:"numbers"`
	}
	run := func(args ...string) report {
		var out, errOut bytes.Buffer
		if rc := runReport(append([]string{"--json"}, args...), &out, &errOut); rc != 0 {
			t.Fatalf("rc=%d %s", rc, errOut.String())
		}
		var r report
		if err := json.Unmarshal(out.Bytes(), &r); err != nil {
			t.Fatal(err)
		}
		return r
	}

	win := run("--days=30")
	if win.CompactorSavedBytes != 300 || win.Numbers.Compactor.SavedBytes != 300 || win.Numbers.Compactor.Tokens != 75 || win.TokensSavedEstimate != 75 {
		t.Errorf("30-day report = %+v, want the recent run only (300 bytes, 75 tokens)", win)
	}
	all := run("--days=0")
	if all.CompactorSavedBytes != 4300 || all.Numbers.Compactor.SavedBytes != 4300 || all.TokensSavedEstimate != 1075 {
		t.Errorf("all-time report = %+v, want both runs (4300 bytes, 1075 tokens)", all)
	}
}

// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/execwrap"
)

// The stats headline opens with the token ledger: spent, saved by routing and
// saved by the compactor (labelled as an estimate), over the --days window.
// A compactor run alone is enough to print it.
func TestStats_HeadlineIsTheTokenLedger(t *testing.T) {
	endEnv(t)
	path, err := execwrap.EventsPath()
	if err != nil {
		t.Fatal(err)
	}
	ev := execwrap.Event{TS: time.Now().UTC().Add(-time.Hour).Format(time.RFC3339), RawBytes: 4100, EmittedBytes: 100, Compacted: true}
	if err := execwrap.Append(path, ev); err != nil {
		t.Fatal(err)
	}

	var rc int
	out := captureStdout(func() { rc = runStats([]string{"--days=7"}) })
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	first := strings.SplitN(out, "\n", 2)[0]
	want := "Token ledger (last 7 days): 0 spent · 0 saved by routing · 1.0k saved by compactor (estimate: saved bytes / 4)"
	if first != want {
		t.Fatalf("headline = %q, want %q", first, want)
	}

	out = captureStdout(func() { rc = runStats([]string{"--days=7", "--export"}) })
	if rc != 0 {
		t.Fatalf("--export rc=%d", rc)
	}
	var exp struct {
		Total  *int `json:"total"`
		Ledger struct {
			CompactorSaved int64  `json:"compactor_saved"`
			Estimate       string `json:"compactor_estimate"`
		} `json:"token_ledger"`
	}
	if err := json.Unmarshal([]byte(out), &exp); err != nil {
		t.Fatalf("--export is not JSON: %v\n%s", err, out)
	}
	if exp.Total == nil || exp.Ledger.CompactorSaved != 1000 || exp.Ledger.Estimate != "estimate: saved bytes / 4" {
		t.Errorf("--export = %s, want the summary keys plus token_ledger", out)
	}
}

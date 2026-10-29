// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
	"github.com/tiagovilasboas/downshift/internal/workers"
)

// Worker tokens show per CLI under the ledger headline, in stats --export
// and in report --json numbers.ledger.
func TestStats_ShowsWorkerTokensPerCLI(t *testing.T) {
	endEnv(t)
	telemetry.Record(workerEndEvent(workers.End{Agent: "claude", Outcome: workers.Done,
		Tokens: &workers.Tokens{Input: 1000, Output: 500, CacheRead: 200, CacheWrite: 300}}))
	telemetry.Record(workerEndEvent(workers.End{Agent: "codex", Outcome: workers.Done,
		Tokens: &workers.Tokens{Input: 40, Output: 10}}))

	var rc int
	out := captureStdout(func() { rc = runStats([]string{"--days=7"}) })
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	lines := strings.Split(out, "\n")
	if want := "Workers (additional to spent): claude 2.0k · codex 50"; lines[1] != want {
		t.Fatalf("line 2 = %q, want %q\n%s", lines[1], want, out)
	}

	out = captureStdout(func() { rc = runStats([]string{"--days=7", "--export"}) })
	var exp struct {
		Ledger struct {
			Workers []struct {
				Agent string `json:"agent"`
				Total int64  `json:"total"`
			} `json:"workers"`
		} `json:"token_ledger"`
	}
	if err := json.Unmarshal([]byte(out), &exp); err != nil || len(exp.Ledger.Workers) != 2 ||
		exp.Ledger.Workers[0].Agent != "claude" || exp.Ledger.Workers[0].Total != 2000 {
		t.Errorf("--export workers = %s (err %v)", out, err)
	}

	var rep, errOut bytes.Buffer
	if rc := runReport([]string{"--json"}, &rep, &errOut); rc != 0 {
		t.Fatalf("report rc=%d %s", rc, errOut.String())
	}
	var rj struct {
		Numbers struct {
			Ledger struct {
				Workers []struct {
					Agent string `json:"agent"`
					Total int64  `json:"total"`
				} `json:"workers"`
			} `json:"ledger"`
		} `json:"numbers"`
	}
	if err := json.Unmarshal(rep.Bytes(), &rj); err != nil || len(rj.Numbers.Ledger.Workers) != 2 ||
		rj.Numbers.Ledger.Workers[1].Agent != "codex" || rj.Numbers.Ledger.Workers[1].Total != 50 {
		t.Errorf("report numbers.ledger.workers = %s (err %v)", rep.String(), err)
	}
}

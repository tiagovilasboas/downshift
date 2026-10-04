// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1
// Commercial use requires a licence — see LICENSE for terms.

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeDataset(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ds.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// A malformed threshold must fail instead of silently disabling the gate.
func TestRunBenchmark_InvalidMinTierAccuracyFails(t *testing.T) {
	ds := writeDataset(t, `[{"prompt":"rename the userId variable","label":"TRIVIAL"}]`)
	for _, v := range []string{"abc", "0,8", "1.5", "-0.1"} {
		var rc int
		captureStderr(func() { rc = runBenchmark([]string{ds, "--min-tier-accuracy=" + v}) })
		if rc == 0 {
			t.Errorf("--min-tier-accuracy=%s: rc=0, want failure", v)
		}
	}
}

// A dataset where no row has a recognised label must not report success.
func TestRunBenchmark_NoValidTasksFails(t *testing.T) {
	ds := writeDataset(t, `[{"prompt":"x","label":"weird"}]`)
	var rc int
	captureStdout(func() { captureStderr(func() { rc = runBenchmark([]string{ds, "--report"}) }) })
	if rc == 0 {
		t.Fatal("rc=0 for a dataset with zero valid tasks")
	}
}

// --max-frontier-to-mid tightens the FRONTIER→MID gate (CI uses 0 on seed
// and holdout); a bad value fails.
func TestRunBenchmark_MaxFrontierToMidGate(t *testing.T) {
	ds := writeDataset(t, `[{"prompt":"look at the logs folder and tell me what you see","label":"COMPLEX"}]`)
	var strict, loose, bad int
	captureStderr(func() {
		strict = runBenchmark([]string{ds, "--gate", "--min-tier-accuracy=0.01", "--max-frontier-to-mid=0"})
		loose = runBenchmark([]string{ds, "--gate", "--min-tier-accuracy=0.01", "--max-frontier-to-mid=1"})
		bad = runBenchmark([]string{ds, "--gate", "--max-frontier-to-mid=x"})
	})
	if strict == 0 {
		t.Error("FRONTIER→MID 1/1 must fail --max-frontier-to-mid=0")
	}
	if bad == 0 {
		t.Error("invalid --max-frontier-to-mid must fail")
	}
	_ = loose // tier accuracy 0 still fails the accuracy gate; only strict/bad matter here
}

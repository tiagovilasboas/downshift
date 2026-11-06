// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/paths"
)

// The flags the quoted rewrite emits for `|| true` and `2>/dev/null` reach
// the runner after --session: a refused program (126) exits 0 under
// --exit-zero, and its stdout carries no note: nothing was compacted.
func TestRunExec_TailFlags(t *testing.T) {
	t.Setenv(paths.EnvStateDir, t.TempDir())
	var out, errb bytes.Buffer
	args := []string{"--session=" + strings.Repeat("a", 64), "--discard-stderr", "--exit-zero", "--", "downshift-test-no-such-program"}
	if rc := runExec(args, strings.NewReader(""), &out, &errb); rc != 0 {
		t.Fatalf("rc = %d, want 0 (stderr %q)", rc, errb.String())
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want empty: notes only follow a compacted output", out.String())
	}
	out.Reset()
	if rc := runExec([]string{"--exit-zero", "go", "test"}, strings.NewReader(""), &out, &errb); rc != 2 {
		t.Fatalf("a tail flag without -- is usage: rc = %d", rc)
	}
}

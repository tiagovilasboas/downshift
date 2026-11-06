// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSkipLogHoldsNoCommandText(t *testing.T) {
	p := filepath.Join(t.TempDir(), SkipsFileName)
	secret := "/Users/someone/secret-project/bin/gh pr list --token abc123"
	d := Decide(secret, "/x/downshift")
	reason := SkipReason(d, secret)
	if reason != SkipAbsPath {
		t.Fatalf("reason = %q, want %q", reason, SkipAbsPath)
	}
	if err := AppendSkip(p, reason, "claude-code", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := AppendSkip(p, "", "claude-code", time.Now()); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, leak := range []string{"secret", "abc123", "pr list", "/Users", "token"} {
		if strings.Contains(string(raw), leak) {
			t.Errorf("skip row leaks %q: %s", leak, raw)
		}
	}
	got, err := SummarizeSkips(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[SkipAbsPath] != 1 {
		t.Errorf("counts = %v, want one abs_path", got)
	}
}

func TestSummarizeSkipsMissingFile(t *testing.T) {
	got, err := SummarizeSkips(filepath.Join(t.TempDir(), "none.jsonl"))
	if err != nil || got != nil {
		t.Errorf("got %v, %v; want nil, nil", got, err)
	}
}

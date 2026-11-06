// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// codex_readonly_refused.jsonl is the sanitized stdout of one real run of
// codex-cli 0.163.0 (docs/evidence/mcp-codex-2026-11-06.md): the read-only
// recipe was asked to write a file and refused. It carries two item.completed
// events of type "error" (a user-config warning) that neither parser may trip on.
const codexRefusedFixture = "codex_readonly_refused.jsonl"

func readCodexFixture(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", codexRefusedFixture))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestCodexFixtureCarriesTheErrorEventsTheParsersMustSkip(t *testing.T) {
	raw := string(readCodexFixture(t))
	if n := strings.Count(raw, `"type":"error"`); n != 2 {
		t.Fatalf("fixture has %d error items, want 2", n)
	}
	if strings.Contains(raw, "/Users/") {
		t.Fatal("fixture holds a real home path")
	}
}

func TestCodexSessionReadsTheThreadIDOfARealRun(t *testing.T) {
	const want = "01a1b297-23d2-76d3-adf4-bad7bfb5dc07"
	if got := codexSession(readCodexFixture(t)); got != want {
		t.Fatalf("codexSession = %q, want %q", got, want)
	}
}

func TestCodexTokensOfARealRunTakeCachedInputOutOfInput(t *testing.T) {
	// input_tokens 56475 includes cached_input_tokens 28032, so Input is 28443.
	want := Tokens{Input: 28443, Output: 71, CacheRead: 28032, CacheWrite: 0, Total: 56546}
	got := codexTokens(readCodexFixture(t))
	if got == nil || *got != want {
		t.Fatalf("codexTokens = %+v, want %+v", got, want)
	}
}

func TestCodexParsersIgnoreTheErrorEventsOnTheirOwn(t *testing.T) {
	var errorsOnly []string
	for _, l := range strings.Split(string(readCodexFixture(t)), "\n") {
		if strings.Contains(l, `"type":"error"`) {
			errorsOnly = append(errorsOnly, l)
		}
	}
	in := []byte(strings.Join(errorsOnly, "\n") + "\n")
	if got := codexSession(in); got != "" {
		t.Fatalf("codexSession on error events = %q, want empty", got)
	}
	if got := codexTokens(in); got != nil {
		t.Fatalf("codexTokens on error events = %+v, want nil", got)
	}
}

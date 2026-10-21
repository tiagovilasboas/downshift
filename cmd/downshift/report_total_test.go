// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
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
	if last := lines[len(lines)-1]; last != "Tokens saved (estimate): 0" {
		t.Fatalf("last line = %q, want the final estimate", last)
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
}

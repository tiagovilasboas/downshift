// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/accounting"
)

// The report shows the top passthrough details and metachar classes in both
// the markdown and the plain-text forms, and nothing when a harness has none.
func TestCompactionDetailShowsCodes(t *testing.T) {
	d := accounting.CompactionDetail{Harnesses: []accounting.HarnessCompaction{{
		Harness: "claude-code", Commands: 3,
		PassthroughDetails: []accounting.ReasonCount{{Reason: "cr", Count: 2}},
		SkipMetachars:      []accounting.ReasonCount{{Reason: "pipe", Count: 5}},
	}, {Harness: "kirocrew"}}}
	var md, txt bytes.Buffer
	printCompactionDetailMarkdown(&md, d)
	printCompactionDetailText(&txt, d)
	for name, out := range map[string]string{"markdown": md.String(), "text": txt.String()} {
		for _, want := range []string{"carriage return", "| pipe"} {
			if !strings.Contains(out, want) {
				t.Errorf("%s output lacks %q:\n%s", name, want, out)
			}
		}
	}
	if strings.Count(md.String(), "metacharacter") != 1 {
		t.Errorf("harness with no codes printed a table:\n%s", md.String())
	}
}

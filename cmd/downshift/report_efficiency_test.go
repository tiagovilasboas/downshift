// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/accounting"
)

func efficiencyOutputs(t *testing.T) (md, text, js string) {
	t.Helper()
	whyFixtureEnv(t)
	seedCompactorAndQuota(t)
	return runReportOut(t, "--days=0"), runReportOut(t, "--days=0", "--text"), runReportOut(t, "--days=0", "--json")
}

// efficiencySlice is the section from heading up to the next one.
func efficiencySlice(t *testing.T, text, heading, next string) string {
	t.Helper()
	s := sectionOf(t, text, heading)
	if i := strings.Index(s[len(heading):], next); i >= 0 {
		s = s[:len(heading)+i]
	}
	return s
}

func TestReport_EfficiencySectionInAllFormats(t *testing.T) {
	md, text, js := efficiencyOutputs(t)
	if !strings.Contains(md, "## Efficiency per harness") {
		t.Errorf("markdown lacks the Efficiency section:\n%s", md)
	}
	if i, j := strings.Index(md, "## Compaction savings per harness"), strings.Index(md, "## Efficiency per harness"); i < 0 || j < i {
		t.Errorf("Efficiency must follow the Compactor section (%d, %d)", i, j)
	}
	if !strings.Contains(text, "EFFICIENCY PER HARNESS") {
		t.Errorf("text lacks the Efficiency section:\n%s", text)
	}
	var rep struct {
		Efficiency accounting.Efficiency `json:"efficiency"`
	}
	if err := json.Unmarshal([]byte(js), &rep); err != nil {
		t.Fatal(err)
	}
	if len(rep.Efficiency.Harnesses) == 0 {
		t.Fatalf("--json efficiency has no harness:\n%s", js)
	}
	for _, h := range rep.Efficiency.Harnesses {
		if h.Spawns == 0 {
			t.Errorf("harness %s listed without routed spawns", h.Harness)
		}
	}
}

// A harness with no completed task reads "unobserved" (codex in the fixture),
// never 0%, and the Compactor tokens are not printed again here.
func TestReport_EfficiencyUnobservedAndNoRepeatedTokens(t *testing.T) {
	md, _, _ := efficiencyOutputs(t)
	section := efficiencySlice(t, md, "## Efficiency per harness", "\n## ")
	if !strings.Contains(section, "| codex | unobserved |") || strings.Contains(section, "0% (0 of 0 completed") {
		t.Errorf("premium preserved must read unobserved:\n%s", section)
	}
	if strings.Contains(section, "tokens avoided") {
		t.Errorf("Efficiency repeats the Compactor tokens:\n%s", section)
	}
}

func TestReport_EfficiencyHasNoMoney(t *testing.T) {
	md, text, js := efficiencyOutputs(t)
	var rep map[string]json.RawMessage
	if err := json.Unmarshal([]byte(js), &rep); err != nil {
		t.Fatal(err)
	}
	section := efficiencySlice(t, md, "## Efficiency per harness", "\n## ") + "\n" +
		efficiencySlice(t, text, "EFFICIENCY PER HARNESS", "\n\n") + "\n" + string(rep["efficiency"])
	lower := strings.ToLower(section)
	for _, banned := range []string{"$", "cost", "price", "usd", "dollar"} {
		if strings.Contains(lower, banned) {
			t.Errorf("the Efficiency section mentions %q:\n%s", banned, section)
		}
	}
}

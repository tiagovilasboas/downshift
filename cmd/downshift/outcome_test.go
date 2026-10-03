package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/tiagovilasboas/harness-downshift/internal/outcome"
)

// README numbers come from `downshift eval-outcome --report`; this fails when
// the generated block in README.md drifts from what the code produces.
func TestREADMEReportInSync(t *testing.T) {
	block, err := buildReport("../..", "benchmark/outcomes/testdata", "benchmark/outcomes/runs")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	updated, err := outcome.Splice(string(doc), block)
	if err != nil {
		t.Fatal(err)
	}
	if updated != string(doc) {
		t.Fatal("README.md report block is stale: run `go run ./cmd/downshift eval-outcome --report --write README.md`")
	}
}

func TestEvalOutcomeRecordRefusedInCI(t *testing.T) {
	t.Setenv("CI", "true")
	var out, errb bytes.Buffer
	code := runEvalOutcome([]string{"--record", "--tier=small", "--model=x", "--solver=true"}, &out, &errb)
	if code != 2 || !strings.Contains(errb.String(), "disabled when CI is set") {
		t.Fatalf("code=%d stderr=%q", code, errb.String())
	}
}

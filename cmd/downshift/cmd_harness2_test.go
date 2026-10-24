// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/harness2export"
)

func TestRunHarness2MatrixJSON(t *testing.T) {
	out := captureStdout(func() {
		if rc := runHarness2([]string{"matrix", "--json"}); rc != 0 {
			t.Fatalf("rc = %d", rc)
		}
	})
	var doc struct {
		SchemaVersion int `json:"schema_version"`
		Harnesses     []struct {
			ID string `json:"id"`
		} `json:"harnesses"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("json: %v\n%s", err, out)
	}
	if doc.SchemaVersion != 1 || len(doc.Harnesses) < 6 {
		t.Fatalf("unexpected payload: %+v", doc)
	}
}

func TestRunHarness2EvalPublicDataset(t *testing.T) {
	dataset := filepath.Join("..", "..", "benchmark", "routing-tasks.public.json")
	var rc int
	out := captureStdout(func() {
		rc = runHarness2([]string{"eval", dataset, "--json"})
	})
	if rc != 0 {
		t.Fatalf("rc = %d stdout=%s", rc, out)
	}
	if !strings.Contains(out, "tier_accuracy") {
		t.Fatalf("missing metrics in report: %s", out)
	}
}

func TestRunHarness2LabReceipt(t *testing.T) {
	if rc := runHarness2([]string{"lab"}); rc != 0 {
		t.Fatalf("rc = %d", rc)
	}
	root, err := downshiftModuleRoot()
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(labReceiptRel)))
	if err != nil {
		t.Fatal(err)
	}
	var got harness2export.LabReceipt
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	want, err := harness2export.BuildReceipt(harness2MatrixRows())
	if err != nil {
		t.Fatal(err)
	}
	if got.SmokePromotionGate || !want.JudgePassed {
		t.Fatalf("smoke gate or judge: %+v", got)
	}
	if len(got.MutationTargets) != 1 || got.MutationTargets[0] != "claude-code" {
		t.Fatalf("mutation targets: %v", got.MutationTargets)
	}
	for _, id := range got.ObservationTargets {
		if id == "claude-code" {
			t.Fatalf("observation list includes mutation target: %v", got.ObservationTargets)
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("receipt drift:\n got %#v\nwant %#v", got, want)
	}
}

// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

// Classifier stability guard. TestPromptCorpusFiles asserts the tier each
// prompt should get; this test pins what the router decides for every checked-in
// corpus prompt (complexity, confidence, tier), so any behavioural drift, even
// one that stays inside the label, shows up as a reviewed diff of
// testdata/stability/golden.json instead of a silent change. It replaces the
// local-only check that read a private baseline directory.
//
// Intentional change? Re-run with the update flag and review the diff:
//
//	go test ./internal/core -run TestClassifierStability -update-stability
//
// Downshift by Tiago Vilas Boas
// https://github.com/tiagovilasboas/downshift

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

var updateStability = flag.Bool("update-stability", false, "rewrite testdata/stability/golden.json")

const stabilityGolden = "testdata/stability/golden.json"

type stabilityRow struct {
	Complexity string `json:"complexity"`
	Confident  bool   `json:"confident"`
	Tier       string `json:"tier"`
}

func TestClassifierStability(t *testing.T) {
	got := map[string]stabilityRow{}
	for file, cases := range loadPromptCorpus(t) {
		for _, c := range cases {
			d := classifiedRoute(c.Prompt, "claude-code", "", cat)
			got[file+"/"+c.ID] = stabilityRow{d.Complexity.String(), d.Confident, d.Tier.String()}
		}
	}
	if *updateStability {
		b, err := json.MarshalIndent(got, "", " ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(stabilityGolden), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(stabilityGolden, append(b, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %d rows", len(got))
		return
	}
	raw, err := os.ReadFile(stabilityGolden)
	if err != nil {
		t.Fatalf("%v (create it with -update-stability)", err)
	}
	var want map[string]stabilityRow
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	var ids []string
	for id := range got {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	drift := 0
	for _, id := range ids {
		w, ok := want[id]
		if !ok {
			t.Errorf("%s: not in the golden file (new corpus prompt; run -update-stability)", id)
			drift++
		} else if got[id] != w {
			t.Errorf("%s: decision changed: got %+v, golden %+v", id, got[id], w)
			drift++
		}
	}
	for id := range want {
		if _, ok := got[id]; !ok {
			t.Errorf("%s: in the golden file but not in the corpus", id)
			drift++
		}
	}
	if drift > 0 {
		t.Fatalf("%d decisions differ from %s; if intended, run -update-stability and review the diff with before/after evidence", drift, stabilityGolden)
	}
}

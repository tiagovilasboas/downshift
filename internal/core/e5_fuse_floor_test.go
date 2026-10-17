// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/semantic"
	"github.com/tiagovilasboas/downshift/internal/semantic/e5shadow"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// A prompt the in-process hash boost lifts from an unconfident MEDIUM to
// COMPLEX with the fuse off (regex Complex score present, so the lift is
// allowed). Found by scanning the seed set; the test checks the precondition.
const hashLiftPrompt = "implement end-to-end encryption for the messaging feature across web and mobile"

func hasReason(d core.Decision, code string) bool {
	for _, c := range telemetry.ReasonCodes(d) {
		if c == code {
			return true
		}
	}
	return false
}

// Enable gate 2 forbids a fuse-on class below the fuse-off class. With the
// fuse on, the external MiniLM helper is skipped, so the hash lift that
// fuse-off applies must survive as a floor, credited to the hash boost.
func TestE5Fuse_FuseOnNeverEndsBelowFuseOff(t *testing.T) {
	e5FuseHook(t)
	t.Setenv("DOWNSHIFT_MINILM", "")
	t.Setenv("DOWNSHIFT_MINILM_EMBED", "")
	base := core.Classify(hashLiftPrompt)
	off := core.ClassifyWithSemantic(hashLiftPrompt)
	if base.Confident || off.Complexity <= base.Complexity {
		t.Skipf("precondition changed: base=%v/%v off=%v", base.Complexity, base.Confident, off.Complexity)
	}

	// Helper answers, nearest centroid is the regex class itself: E5 would not lift.
	t.Setenv(e5shadow.IntegrateEnv, "1")
	t.Setenv(e5shadow.CmdEnvAlias, "/bin/echo [1,0]")
	t.Setenv(e5shadow.PrototypesEnv, writeE5Prototypes(t, semantic.PrototypeStore{
		Dim: 2,
		Centroids: map[string][]float64{
			base.Complexity.String(): {1, 0},
			semantic.LabelComplex:    {0, 1},
		},
	}))
	on := core.ClassifyWithSemantic(hashLiftPrompt)
	if !on.E5Fuse.Enabled || on.E5Fuse.ErrorCode != "" {
		t.Fatalf("fuse did not run: %+v", on.E5Fuse)
	}
	if on.Complexity < off.Complexity {
		t.Fatalf("fuse on ended below fuse off: on=%v off=%v", on.Complexity, off.Complexity)
	}
	if on.Confident || !on.SemanticLift {
		t.Fatalf("floored lift must clear confidence and mark the semantic lift: %+v", on)
	}
	if on.E5Fuse.Action != core.E5ActionNone {
		t.Fatalf("the floor's lift is the hash boost's, not E5's: action=%q", on.E5Fuse.Action)
	}
	d := core.Route(hashLiftPrompt, "claude-code", "")
	if !hasReason(d, telemetry.ReasonSemanticBoost) || hasReason(d, telemetry.ReasonE5Boost) {
		t.Fatalf("floored lift must log SEMANTIC_BOOST, not E5_BOOST: %v", telemetry.ReasonCodes(d))
	}
}

func TestE5Fuse_TraceOffIsEmpty(t *testing.T) {
	e5FuseHook(t)
	d := core.Route("rename la variable contraseña", "claude-code", "")
	if d.E5Fuse.Enabled || d.E5Fuse != (core.E5Fuse{}) {
		t.Fatalf("flag off must leave an empty trace: %+v", d.E5Fuse)
	}
	ev := telemetry.FromDecision(d, "c", "test")
	if ev.E5Fuse != nil {
		t.Fatalf("flag off must write no e5_fuse object: %+v", ev.E5Fuse)
	}
}

func TestE5Fuse_TraceRecordsFailOpenCode(t *testing.T) {
	e5FuseHook(t)
	t.Setenv(e5shadow.IntegrateEnv, "1")
	d := core.Route("rename la variable contraseña", "claude-code", "")
	if !d.E5Fuse.Enabled || d.E5Fuse.ErrorCode != "missing_cmd" || d.E5Fuse.Action != "" {
		t.Fatalf("fail-open trace = %+v", d.E5Fuse)
	}
	if hasReason(d, telemetry.ReasonE5Boost) || hasReason(d, telemetry.ReasonE5Abstain) {
		t.Fatalf("fail-open must add no E5 reason code: %v", telemetry.ReasonCodes(d))
	}
	ev := telemetry.FromDecision(d, "c", "test")
	if ev.E5Fuse == nil || ev.E5Fuse.ErrorCode != "missing_cmd" {
		t.Fatalf("e5_fuse record = %+v", ev.E5Fuse)
	}
}

func TestE5Fuse_RaiseLogsE5BoostNotSemanticBoost(t *testing.T) {
	e5FuseHook(t)
	prompt := "rename la variable contraseña"
	base := core.Classify(prompt)
	if base.Confident || base.Complexity != core.Trivial {
		t.Skipf("precondition changed: %+v", base)
	}
	t.Setenv(e5shadow.IntegrateEnv, "1")
	t.Setenv(e5shadow.CmdEnvAlias, "/bin/echo [0,1]")
	t.Setenv(e5shadow.PrototypesEnv, writeE5Prototypes(t, semantic.PrototypeStore{
		Dim: 2,
		Centroids: map[string][]float64{
			semantic.LabelTrivial: {1, 0},
			semantic.LabelSimple:  {0, 1},
		},
	}))
	d := core.Route(prompt, "claude-code", "")
	if d.E5Fuse.Action != core.E5ActionRaise || d.Complexity != core.Simple || d.Confident {
		t.Fatalf("decision = complexity %v confident %v trace %+v", d.Complexity, d.Confident, d.E5Fuse)
	}
	if !hasReason(d, telemetry.ReasonE5Boost) || hasReason(d, telemetry.ReasonSemanticBoost) {
		t.Fatalf("an E5 raise is E5_BOOST and not SEMANTIC_BOOST: %v", telemetry.ReasonCodes(d))
	}
	if d.E5Fuse.NearestLabel != semantic.LabelSimple || d.E5Fuse.ModelID == "" || d.E5Fuse.Similarity < semantic.MinSimilarity {
		t.Fatalf("trace fields = %+v", d.E5Fuse)
	}
}

func TestE5Fuse_AbstainLogsE5Abstain(t *testing.T) {
	e5FuseHook(t)
	prompt := "rename foo to bar"
	base := core.Classify(prompt)
	t.Setenv(e5shadow.IntegrateEnv, "1")
	t.Setenv(e5shadow.CmdEnvAlias, "/bin/echo [0,1]")
	t.Setenv(e5shadow.PrototypesEnv, writeE5Prototypes(t, semantic.PrototypeStore{
		Dim: 2,
		Centroids: map[string][]float64{
			semantic.LabelTrivial: {1, 0},
			semantic.LabelForeign: {0, 1},
		},
	}))
	d := core.Route(prompt, "claude-code", "")
	if d.E5Fuse.Action != core.E5ActionAbstain || d.Confident || d.Complexity != base.Complexity {
		t.Fatalf("abstain must clear confidence only: complexity %v (base %v) confident %v trace %+v", d.Complexity, base.Complexity, d.Confident, d.E5Fuse)
	}
	if !hasReason(d, telemetry.ReasonE5Abstain) || hasReason(d, telemetry.ReasonE5Boost) || hasReason(d, telemetry.ReasonSemanticBoost) {
		t.Fatalf("reason codes = %v", telemetry.ReasonCodes(d))
	}
}

// Enable gate 7: the e5_fuse object is a closed field set and never carries
// the prompt, the vector, the helper's stderr or an env path.
func TestE5Fuse_RecordIsClosedAndPromptFree(t *testing.T) {
	e5FuseHook(t)
	prompt := "rename la variable contraseña"
	protoPath := writeE5Prototypes(t, semantic.PrototypeStore{
		Dim: 2,
		Centroids: map[string][]float64{
			semantic.LabelTrivial: {1, 0},
			semantic.LabelSimple:  {0, 1},
		},
	})
	t.Setenv(e5shadow.IntegrateEnv, "1")
	t.Setenv(e5shadow.CmdEnvAlias, "/bin/echo [0,1]")
	t.Setenv(e5shadow.PrototypesEnv, protoPath)
	ev := telemetry.FromDecision(core.Route(prompt, "claude-code", ""), "c", "test")
	raw, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(top["e5_fuse"], &fields); err != nil {
		t.Fatalf("e5_fuse missing or malformed: %s", raw)
	}
	allowed := map[string]bool{"model_id": true, "nearest_label": true, "similarity": true, "action": true, "latency_ms": true, "error_code": true}
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
		if !allowed[k] {
			t.Errorf("e5_fuse carries a field outside the closed set: %q", k)
		}
	}
	sort.Strings(keys)
	if fields["action"] != core.E5ActionRaise {
		t.Fatalf("action = %v, keys %v", fields["action"], keys)
	}
	s := string(raw)
	for _, forbidden := range []string{"contraseña", "/bin/echo", protoPath, "[0,1]"} {
		if strings.Contains(s, forbidden) {
			t.Errorf("event leaks %q: %s", forbidden, s)
		}
	}
}

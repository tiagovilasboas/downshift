// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

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

func e5FuseHook(t *testing.T) {
	t.Helper()
	t.Setenv("DOWNSHIFT_MINILM", "0")
	t.Setenv(e5shadow.IntegrateEnv, "")
	t.Setenv(e5shadow.EnableEnv, "")
	t.Setenv(e5shadow.CmdEnv, "")
	t.Setenv(e5shadow.CmdEnvAlias, "")
	t.Setenv(e5shadow.PrototypesEnv, "")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	restore := hookctx.Set(ctx)
	t.Cleanup(func() {
		restore()
		cancel()
	})
}

func writeE5Prototypes(t *testing.T, store semantic.PrototypeStore) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "prototypes.json")
	data, err := json.Marshal(store)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestE5Fuse_FlagOffUnchanged(t *testing.T) {
	e5FuseHook(t)
	prompt := "rename the variable userId to userIdentifier"
	base := core.Classify(prompt)
	t.Setenv(e5shadow.IntegrateEnv, "")
	t.Setenv(e5shadow.CmdEnv, "/bin/echo [0,1]")
	t.Setenv(e5shadow.PrototypesEnv, writeE5Prototypes(t, semantic.PrototypeStore{
		Dim: 2,
		Centroids: map[string][]float64{
			semantic.LabelTrivial: {1, 0},
			semantic.LabelSimple:  {0, 1},
		},
	}))
	got := core.ClassifyWithSemantic(prompt)
	if got.Complexity != base.Complexity || got.Confident != base.Confident || got.RiskAbstain != base.RiskAbstain {
		t.Fatalf("flag off changed classification: base=%+v got=%+v", base, got)
	}
}

func TestE5Fuse_FailOpenUnchanged(t *testing.T) {
	e5FuseHook(t)
	prompt := "rename the variable userId to userIdentifier"
	base := core.Classify(prompt)
	t.Setenv(e5shadow.IntegrateEnv, "1")
	t.Setenv(e5shadow.CmdEnv, "")
	t.Setenv(e5shadow.CmdEnvAlias, "")
	got := core.ClassifyWithSemantic(prompt)
	if got.Complexity != base.Complexity || got.Confident != base.Confident {
		t.Fatalf("fail-open changed classification: base=%+v got=%+v", base, got)
	}
}

func TestE5Fuse_SuccessCanRaise(t *testing.T) {
	e5FuseHook(t)
	// Spanish mixed rename: language guard clears confidence, risk table
	// does not floor it, so a SIMPLE centroid is an allowed monotonic raise.
	prompt := "rename la variable contraseña"
	base := core.Classify(prompt)
	if base.NoSignal || base.Confident || !base.UnsupportedLanguage || base.Complexity != core.Trivial {
		t.Fatalf("precondition: want unconfident TRIVIAL with language guard, got %+v", base)
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
	got := core.ClassifyWithSemantic(prompt)
	if got.Complexity.Tier() < base.Complexity.Tier() {
		t.Fatalf("E5 fuse must not downgrade: base=%v got=%v", base.Complexity, got.Complexity)
	}
	if got.Complexity == base.Complexity {
		t.Fatalf("expected fixture-controlled raise, base=%v got=%v", base.Complexity, got.Complexity)
	}
	if got.Confident {
		t.Fatal("language-mismatch abstain must keep Confident=false")
	}
}

func TestE5Fuse_NoSignalIgnored(t *testing.T) {
	e5FuseHook(t)
	prompt := "zzz qqq www"
	base := core.Classify(prompt)
	if !base.NoSignal {
		t.Fatalf("precondition: want NoSignal, got %+v", base)
	}
	t.Setenv(e5shadow.IntegrateEnv, "1")
	t.Setenv(e5shadow.CmdEnvAlias, "/bin/echo [0,1]")
	t.Setenv(e5shadow.PrototypesEnv, writeE5Prototypes(t, semantic.PrototypeStore{
		Dim: 2,
		Centroids: map[string][]float64{
			semantic.LabelTrivial: {1, 0},
			semantic.LabelComplex: {0, 1},
		},
	}))
	got := core.ClassifyWithSemantic(prompt)
	if got.Complexity != base.Complexity || got.NoSignal != base.NoSignal {
		t.Fatalf("no-signal prompt must ignore E5: base=%+v got=%+v", base, got)
	}
}

func TestE5Fuse_NoComplexFromSemanticAlone(t *testing.T) {
	e5FuseHook(t)
	prompt := h14MixedRiskPrompts()[0]
	base := core.Classify(prompt)
	if base.Scores[core.Complex] != 0 {
		t.Fatalf("precondition: want zero Complex score, got %v", base.Scores)
	}
	t.Setenv(e5shadow.IntegrateEnv, "1")
	t.Setenv(e5shadow.CmdEnvAlias, "/bin/echo [0,1]")
	t.Setenv(e5shadow.PrototypesEnv, writeE5Prototypes(t, semantic.PrototypeStore{
		Dim: 2,
		Centroids: map[string][]float64{
			semantic.LabelTrivial: {1, 0},
			semantic.LabelComplex: {0, 1},
		},
	}))
	got := core.ClassifyWithSemantic(prompt)
	if got.Complexity == core.Complex {
		t.Fatalf("E5 must not sole-source COMPLEX: %+v", got)
	}
}

func TestE5Fuse_LowSimilarityAbstains(t *testing.T) {
	e5FuseHook(t)
	prompt := "rename foo to bar"
	base := core.Classify(prompt)
	if !base.Confident {
		t.Fatalf("precondition: want confident English trivial, got %+v", base)
	}
	t.Setenv(e5shadow.IntegrateEnv, "1")
	t.Setenv(e5shadow.CmdEnvAlias, "/bin/echo [-1,-1]")
	t.Setenv(e5shadow.PrototypesEnv, writeE5Prototypes(t, semantic.PrototypeStore{
		Dim: 2,
		Centroids: map[string][]float64{
			semantic.LabelTrivial: {1, 0},
			semantic.LabelSimple:  {0, 1},
		},
	}))
	got := core.ClassifyWithSemantic(prompt)
	if got.Complexity != base.Complexity || got.Confident != base.Confident {
		t.Fatalf("low similarity must not change regex result: base=%+v got=%+v", base, got)
	}
}

func TestE5Fuse_ReservedLabelAbstains(t *testing.T) {
	e5FuseHook(t)
	prompt := "rename foo to bar"
	base := core.Classify(prompt)
	if !base.Confident {
		t.Fatalf("precondition: want confident English trivial, got %+v", base)
	}
	t.Setenv(e5shadow.IntegrateEnv, "1")
	t.Setenv(e5shadow.CmdEnvAlias, "/bin/echo [0,1]")
	t.Setenv(e5shadow.PrototypesEnv, writeE5Prototypes(t, semantic.PrototypeStore{
		Dim: 2,
		Centroids: map[string][]float64{
			semantic.LabelTrivial: {1, 0},
			semantic.LabelForeign: {0, 1},
		},
	}))
	got := core.ClassifyWithSemantic(prompt)
	if got.Complexity != base.Complexity {
		t.Fatalf("FOREIGN abstain must not change complexity: base=%v got=%v", base.Complexity, got.Complexity)
	}
	if got.Confident {
		t.Fatal("FOREIGN centroid must clear confidence")
	}
	if got.RiskAbstain {
		t.Fatal("E5 must not set RiskAbstain")
	}
}

func TestE5Fuse_H14RiskAbstainStillWins(t *testing.T) {
	e5FuseHook(t)
	offlineRouting(t)
	prompt := h14DelegationBriefPrompt
	base := core.Classify(prompt)
	if !base.RiskAbstain || base.Confident {
		t.Fatalf("precondition: want RiskAbstain, got %+v", base)
	}
	t.Setenv(e5shadow.IntegrateEnv, "1")
	t.Setenv(e5shadow.CmdEnvAlias, "/bin/echo [0,1]")
	t.Setenv(e5shadow.PrototypesEnv, writeE5Prototypes(t, semantic.PrototypeStore{
		Dim: 2,
		Centroids: map[string][]float64{
			semantic.LabelMedium:  {1, 0},
			semantic.LabelComplex: {0, 1},
		},
	}))
	got := core.ClassifyWithSemantic(prompt)
	if got.Confident || !got.RiskAbstain {
		t.Fatalf("H14 RiskAbstain must still win: %+v", got)
	}
	d := core.Route(prompt, "claude-code", catID("claude-code", core.TierFrontier), cat)
	if d.Confident {
		t.Fatalf("Route must not restore confidence: %+v", d)
	}
	if checkWrites(d) && d.Tier < core.TierFrontier && d.Confident {
		t.Fatalf("unsafe confident write: %+v", d)
	}
}

func TestE5Fuse_HungHelperDoesNotChangeSafeVerdict(t *testing.T) {
	e5FuseHook(t)
	prompt := "rename the variable userId to userIdentifier"
	frontier := catID("claude-code", core.TierFrontier)
	off := core.Route(prompt, "claude-code", frontier, cat)

	t.Setenv(e5shadow.IntegrateEnv, "1")
	t.Setenv(e5shadow.CmdEnvAlias, "/bin/sleep 10")
	t.Setenv(e5shadow.CmdEnv, "")
	t.Setenv(e5shadow.PrototypesEnv, writeE5Prototypes(t, semantic.PrototypeStore{
		Dim: 2,
		Centroids: map[string][]float64{
			semantic.LabelTrivial: {1, 0},
			semantic.LabelSimple:  {0, 1},
		},
	}))
	on := core.Route(prompt, "claude-code", frontier, cat)
	if on.Tier != off.Tier || on.Complexity != off.Complexity || on.Confident != off.Confident || on.SafeVerdict != off.SafeVerdict {
		t.Fatalf("hung helper must fail-open: off=%+v on=%+v", off, on)
	}
}

func TestE5Fuse_SharesEmbedWithShadow(t *testing.T) {
	e5FuseHook(t)
	offlineRouting(t)
	dir := t.TempDir()
	countPath := filepath.Join(dir, "count")
	script := filepath.Join(dir, "embed")
	body := "#!/bin/sh\necho x >> " + countPath + "\necho '[0,1]'\n"
	if err := os.WriteFile(script, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	prompt := h14MixedRiskPrompts()[0]
	t.Setenv(e5shadow.IntegrateEnv, "1")
	t.Setenv(e5shadow.EnableEnv, "1")
	t.Setenv(e5shadow.CmdEnv, script)
	t.Setenv(e5shadow.CmdEnvAlias, script)
	t.Setenv(e5shadow.PrototypesEnv, writeE5Prototypes(t, semantic.PrototypeStore{
		Dim: 2,
		Centroids: map[string][]float64{
			semantic.LabelTrivial: {1, 0},
			semantic.LabelSimple:  {0, 1},
		},
	}))
	d := core.Route(prompt, "cursor", "gpt-4", cat)
	if d.Shadow.E5Shadow.ErrorCode != "" {
		t.Fatalf("shadow error: %+v", d.Shadow.E5Shadow)
	}
	data, err := os.ReadFile(countPath)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, b := range data {
		if b == 'x' {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("expected one embed for fuse+shadow, got %d calls (%q)", n, data)
	}
}

func TestE5Fuse_DoesNotReuseShadowCmd(t *testing.T) {
	e5FuseHook(t)
	prompt := "rename the variable userId to userIdentifier"
	base := core.Classify(prompt)
	t.Setenv(e5shadow.IntegrateEnv, "1")
	t.Setenv(e5shadow.CmdEnv, "/bin/echo [0,1]")
	t.Setenv(e5shadow.CmdEnvAlias, "")
	t.Setenv(e5shadow.PrototypesEnv, writeE5Prototypes(t, semantic.PrototypeStore{
		Dim: 2,
		Centroids: map[string][]float64{
			semantic.LabelTrivial: {1, 0},
			semantic.LabelSimple:  {0, 1},
		},
	}))
	got := core.ClassifyWithSemantic(prompt)
	if got.Complexity != base.Complexity || got.Confident != base.Confident {
		t.Fatalf("fuse must not reuse SHADOW_CMD: base=%+v got=%+v", base, got)
	}
}

func TestE5Fuse_ForeignLabelAbstainsWithoutRaise(t *testing.T) {
	e5FuseHook(t)
	prompt := "rename the variable userId to userIdentifier"
	base := core.Classify(prompt)
	if !base.Confident {
		t.Fatalf("precondition: want confident English, got %+v", base)
	}
	t.Setenv(e5shadow.IntegrateEnv, "1")
	t.Setenv(e5shadow.CmdEnvAlias, "/bin/echo [0,1]")
	t.Setenv(e5shadow.PrototypesEnv, writeE5Prototypes(t, semantic.PrototypeStore{
		Dim: 2,
		Centroids: map[string][]float64{
			semantic.LabelTrivial: {1, 0},
			semantic.LabelForeign: {0, 1},
		},
	}))
	look := e5shadow.LookupFuse(prompt)
	if look == nil || look.ErrorCode != "" {
		t.Fatalf("precondition: fuse lookup failed: %+v", look)
	}
	if look.NearestLabel != semantic.LabelForeign {
		t.Fatalf("precondition: nearest=%q want FOREIGN", look.NearestLabel)
	}
	got := core.ClassifyWithSemantic(prompt)
	if got.Complexity != base.Complexity {
		t.Fatalf("FOREIGN must not change complexity: base=%v got=%v", base.Complexity, got.Complexity)
	}
	if got.Confident {
		t.Fatal("FOREIGN must clear confidence")
	}
}

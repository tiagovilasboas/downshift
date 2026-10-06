// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package semantic_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/hookctx"
	"github.com/tiagovilasboas/downshift/internal/semantic"
)

func TestMaybeAugment_OptOut(t *testing.T) {
	t.Setenv("DOWNSHIFT_MINILM", "0")
	out, ok := semantic.MaybeAugment("rename userId", semantic.LabelTrivial, false)
	if ok || out != semantic.LabelTrivial {
		t.Fatalf("expected no change when opted out, got %q ok=%v", out, ok)
	}
}

func TestEmbedderFromEnv_FallsBackWhenCommandFails(t *testing.T) {
	t.Setenv("DOWNSHIFT_MINILM_EMBED", "downshift-missing-embed-binary")
	emb, ok := semantic.EmbedderFromEnv()
	if !ok {
		t.Fatal("expected embedder")
	}
	vec, err := emb.Embed("rename the variable")
	if err != nil || len(vec) == 0 {
		t.Fatalf("fallback hash embed failed: %v len=%d", err, len(vec))
	}
}

func TestAugmentWith_EscalatesWhenUnconfident(t *testing.T) {
	store := semantic.PrototypeStore{
		Dim: 3,
		Centroids: map[string][]float64{
			semantic.LabelTrivial: {1, 0, 0},
			semantic.LabelComplex: {0, 0, 1},
		},
	}
	emb := semantic.StaticEmbedder{Vector: []float64{0, 0, 1}}

	out, ok := semantic.AugmentWith("any", semantic.LabelTrivial, false, store, emb)
	if !ok || out != semantic.LabelComplex {
		t.Fatalf("expected COMPLEX escalation, got %q ok=%v", out, ok)
	}
}

func TestAugmentWith_NoEscalateWhenConfident(t *testing.T) {
	store := semantic.PrototypeStore{
		Dim: 2,
		Centroids: map[string][]float64{
			semantic.LabelTrivial: {1, 0},
			semantic.LabelComplex: {0, 1},
		},
	}
	emb := semantic.StaticEmbedder{Vector: []float64{0, 1}}

	out, ok := semantic.AugmentWith("any", semantic.LabelTrivial, true, store, emb)
	if ok {
		t.Fatalf("confident base should block augment, got %q", out)
	}
}

func TestAugmentWith_NeverDowngrades(t *testing.T) {
	store := semantic.PrototypeStore{
		Dim: 2,
		Centroids: map[string][]float64{
			semantic.LabelTrivial: {1, 0},
			semantic.LabelMedium:  {0.9, 0.1},
		},
	}
	emb := semantic.StaticEmbedder{Vector: []float64{1, 0}}

	out, ok := semantic.AugmentWith("any", semantic.LabelComplex, false, store, emb)
	if ok || out != semantic.LabelComplex {
		t.Fatalf("expected no downgrade from COMPLEX, got %q ok=%v", out, ok)
	}
}

func TestAugmentWith_BelowSimilarityThreshold(t *testing.T) {
	store := semantic.PrototypeStore{
		Dim: 3,
		Centroids: map[string][]float64{
			semantic.LabelTrivial: {1, 0, 0},
			semantic.LabelComplex: {0, 0, 1},
		},
	}
	// Orthogonal to both centroids → cosine 0, below MinSimilarity.
	emb := semantic.StaticEmbedder{Vector: []float64{0, 1, 0}}

	out, ok := semantic.AugmentWith("any", semantic.LabelTrivial, false, store, emb)
	if ok {
		t.Fatalf("expected no augment below threshold, got %q", out)
	}
}

func TestMaybeAugment_EnabledWithHashOnSeedComplex(t *testing.T) {
	t.Setenv("DOWNSHIFT_MINILM", "1")
	t.Setenv("DOWNSHIFT_MINILM_EMBED", "hash")

	prompt := "design and implement gRPC service mesh with service discovery"
	out, ok := semantic.MaybeAugment(prompt, semantic.LabelMedium, false)
	if !ok {
		t.Fatal("expected semantic boost for complex-shaped prompt classified as MEDIUM")
	}
	if semantic.LabelRank(out) <= semantic.LabelRank(semantic.LabelMedium) {
		t.Fatalf("expected rank above MEDIUM, got %q", out)
	}
}

func TestHashEmbed_UnitNorm(t *testing.T) {
	v := semantic.HashEmbed("hello world", 32)
	var sum float64
	for _, x := range v {
		sum += x * x
	}
	if sum < 0.99 || sum > 1.01 {
		t.Fatalf("expected unit norm, got %f", sum)
	}
}

func TestPrototypeStore_Nearest(t *testing.T) {
	store := semantic.PrototypeStore{
		Dim: 2,
		Centroids: map[string][]float64{
			"A": {1, 0},
			"B": {0, 1},
		},
	}
	label, score := store.Nearest([]float64{0.1, 0.9})
	if label != "B" || score < 0.9 {
		t.Fatalf("nearest = %q score=%f", label, score)
	}
}

func TestBuildHashPrototypes_FromTinyDataset(t *testing.T) {
	store := semantic.BuildHashPrototypes(map[string][]string{
		semantic.LabelTrivial: {"rename x"},
		semantic.LabelComplex: {"rearchitect entire platform"},
	}, 16)
	if store.Dim != 16 || len(store.Centroids) != 2 {
		t.Fatalf("unexpected store: dim=%d centroids=%d", store.Dim, len(store.Centroids))
	}
}

func TestLoadPrototypesFromFile(t *testing.T) {
	path := filepath.Join("data", "prototypes.json")
	p, err := semantic.LoadPrototypesFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Dim == 0 || len(p.Centroids) < 4 {
		t.Fatalf("embedded prototypes invalid: dim=%d labels=%d", p.Dim, len(p.Centroids))
	}
}

func TestRefreshHashPrototypesFromBenchmark(t *testing.T) {
	if os.Getenv("REFRESH_PROTOTYPES") != "1" {
		t.Skip("set REFRESH_PROTOTYPES=1 to rewrite prototypes.json")
	}
	err := semantic.RefreshHashPrototypesFromBenchmark("../../internal/benchmark/testdata/sample.json", "data/prototypes.json")
	if err != nil {
		t.Fatal(err)
	}
}

// A hung DOWNSHIFT_MINILM_EMBED helper used to block the hook forever (no
// timeout). It must now fail within the hook deadline.
func TestCmdEmbedder_HungCommandBoundedByHookDeadline(t *testing.T) {
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("sleep not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	defer hookctx.Set(ctx)()
	start := time.Now()
	if _, err := (semantic.CmdEmbedder{Cmd: "sleep 10"}).Embed("prompt"); err == nil {
		t.Fatal("hung embed command must fail")
	}
	if d := time.Since(start); d > time.Second {
		t.Fatalf("embed waited %s past a 300ms deadline", d)
	}
}

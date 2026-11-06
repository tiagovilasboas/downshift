// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"context"
	"testing"
	"time"
)

func TestDefaultsIncludeKiroCLIForKiroCrew(t *testing.T) {
	for _, s := range Defaults() {
		if k, ok := s.(KiroCLI); ok {
			if k.Harness() != "kirocrew" {
				t.Fatalf("KiroCLI harness = %q, want kirocrew", k.Harness())
			}
			return
		}
	}
	t.Fatal("Defaults() has no KiroCLI source: kirocrew would never be discovered")
}

// blockingSource ignores its input until the per-source deadline fires.
type blockingSource struct{}

func (blockingSource) Harness() string { return "blocked" }
func (blockingSource) Name() string    { return "blocking test source" }
func (blockingSource) Discover(ctx context.Context) ([]Model, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

// quickSource answers at once.
type quickSource struct{}

func (quickSource) Harness() string { return "quick" }
func (quickSource) Name() string    { return "quick test source" }
func (quickSource) Discover(context.Context) ([]Model, error) {
	return []Model{{ID: "quick-model"}}, nil
}

func TestRunTimeoutDoesNotStallOtherSources(t *testing.T) {
	start := time.Now()
	out := Run(context.Background(), []Source{blockingSource{}, quickSource{}}, 50*time.Millisecond)
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("Run took %v; the per-source deadline did not bound the slow source", elapsed)
	}
	if len(out) != 2 {
		t.Fatalf("got %d outcomes, want 2", len(out))
	}
	if out[0].Err == nil || len(out[0].Models) != 0 {
		t.Fatalf("slow source must fail with no models, got %+v", out[0])
	}
	if out[1].Err != nil || len(out[1].Models) != 1 || out[1].Models[0].ID != "quick-model" {
		t.Fatalf("fast source must still report, got %+v", out[1])
	}
}

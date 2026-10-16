// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package e5shadow_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/hookctx"
	"github.com/tiagovilasboas/downshift/internal/semantic"
	"github.com/tiagovilasboas/downshift/internal/semantic/e5shadow"
)

func TestObserve_DisabledReturnsNil(t *testing.T) {
	t.Setenv(e5shadow.EnableEnv, "")
	if got := e5shadow.Observe("prompt", semantic.LabelMedium, false); got != nil {
		t.Fatalf("expected nil when disabled, got %+v", got)
	}
}

func TestObserve_MissingCmdFailOpen(t *testing.T) {
	t.Setenv(e5shadow.EnableEnv, "1")
	t.Setenv(e5shadow.CmdEnv, "")
	obs := e5shadow.Observe("prompt", semantic.LabelMedium, false)
	if obs == nil || obs.ErrorCode != "missing_cmd" {
		t.Fatalf("obs = %+v", obs)
	}
	if obs.RegexLabel != semantic.LabelMedium || obs.RegexConfident {
		t.Fatalf("regex context = %+v", obs)
	}
}

func TestObserve_RecordsShadowOpinion(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	restore := hookctx.Set(ctx)
	t.Cleanup(func() {
		restore()
		cancel()
	})

	dir := t.TempDir()
	protoPath := filepath.Join(dir, "prototypes.json")
	store := semantic.PrototypeStore{
		Model: "multilingual-e5-small",
		Dim:   3,
		Centroids: map[string][]float64{
			semantic.LabelTrivial: {1, 0, 0},
			semantic.LabelComplex: {0, 0, 1},
		},
	}
	data, err := json.Marshal(store)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(protoPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	wantID := "e5-prototypes-sha256:" + hex.EncodeToString(digest[:])

	t.Setenv(e5shadow.EnableEnv, "1")
	t.Setenv(e5shadow.CmdEnv, "/bin/echo [0,0,1]")
	t.Setenv(e5shadow.PrototypesEnv, protoPath)

	obs := e5shadow.Observe("any prompt", semantic.LabelTrivial, false)
	if obs == nil {
		t.Fatal("expected observation")
	}
	if obs.ErrorCode != "" {
		t.Fatalf("unexpected error %q", obs.ErrorCode)
	}
	if obs.ModelID != wantID {
		t.Fatalf("model_id = %q, want %q", obs.ModelID, wantID)
	}
	if !strings.HasPrefix(obs.ModelID, "e5-prototypes-sha256:") {
		t.Fatalf("model_id prefix wrong: %q", obs.ModelID)
	}
	if obs.NearestLabel != semantic.LabelComplex {
		t.Fatalf("nearest = %q", obs.NearestLabel)
	}
	if !obs.WouldLift || obs.WouldLabel != semantic.LabelComplex {
		t.Fatalf("would = %q lift=%v", obs.WouldLabel, obs.WouldLift)
	}
}

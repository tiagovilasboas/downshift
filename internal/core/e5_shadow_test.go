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

func TestE5Shadow_EnabledFailOpenDoesNotChangeRoute(t *testing.T) {
	offlineRouting(t)
	t.Setenv("DOWNSHIFT_MINILM", "0")
	t.Setenv(e5shadow.IntegrateEnv, "")
	prompt := "rename userId to accountId across the repo"
	t.Setenv(e5shadow.EnableEnv, "")
	base := core.Route(prompt, "cursor", "gpt-4", cat)

	t.Setenv(e5shadow.EnableEnv, "1")
	t.Setenv(e5shadow.CmdEnv, "")
	on := core.Route(prompt, "cursor", "gpt-4", cat)
	if base.Tier != on.Tier || base.Complexity != on.Complexity || base.Confident != on.Confident {
		t.Fatalf("E5 shadow changed routing: base=%+v on=%+v", base, on)
	}
	if on.E5Shadow.ErrorCode != "missing_cmd" {
		t.Fatalf("expected missing_cmd shadow error, got %+v", on.E5Shadow)
	}
}

func TestE5Shadow_OnRecordsWithoutChangingRoute(t *testing.T) {
	offlineRouting(t)
	t.Setenv("DOWNSHIFT_MINILM", "0")
	t.Setenv(e5shadow.IntegrateEnv, "")
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
	prompt := "rename userId to accountId"
	t.Setenv(e5shadow.EnableEnv, "")
	off := core.Route(prompt, "cursor", "gpt-4", cat)

	t.Setenv(e5shadow.EnableEnv, "1")
	t.Setenv(e5shadow.CmdEnv, "/bin/echo [0,1]")
	t.Setenv(e5shadow.PrototypesEnv, protoPath)
	on := core.Route(prompt, "cursor", "gpt-4", cat)

	if off.Tier != on.Tier || off.Complexity != on.Complexity {
		t.Fatalf("E5 shadow changed tier/complexity: off=%+v on=%+v", off, on)
	}
	if on.E5Shadow.ModelID == "" || on.E5Shadow.ErrorCode != "" {
		t.Fatalf("expected successful shadow record, got %+v", on.E5Shadow)
	}
}

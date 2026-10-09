// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/core"
)

func writeDiscovered(t *testing.T, fetched time.Time, ordered ...string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "discovered.json")
	data, _ := json.Marshal(map[string]any{"harnesses": map[string]any{
		"codex": map[string]any{"fetched_at": fetched, "ordered": ordered},
	}})
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestDiscoveredSessionIsUsedWhenNothingElseKnows(t *testing.T) {
	t.Setenv(core.EnvDiscovery, "")
	t.Setenv(core.EnvDiscovered, writeDiscovered(t, time.Now(), "gpt-6-luna", "gpt-6-sol"))
	got := core.ResolveSession("codex")
	if !got.Known || len(got.IDs) != 2 || got.IDs[1] != "gpt-6-sol" {
		t.Fatalf("got %+v", got)
	}
}

func TestRecoveredSessionBeatsOperatorFile(t *testing.T) {
	t.Setenv(core.EnvDiscovery, "")
	t.Setenv(core.EnvDiscovered, writeDiscovered(t, time.Now(), "gpt-6-luna", "gpt-6-sol"))
	op := filepath.Join(t.TempDir(), "session-models.json")
	_ = os.WriteFile(op, []byte(`{"codex":["only-this"]}`), 0o600)
	t.Setenv("DOWNSHIFT_SESSION_MODELS", op)
	got := core.ResolveSession("codex")
	if len(got.IDs) != 2 || got.IDs[1] != "gpt-6-sol" {
		t.Fatalf("recovered session must beat the operator file, got %+v", got)
	}
}

func TestHookPayloadBeatsDiscovery(t *testing.T) {
	t.Setenv(core.EnvDiscovery, "")
	t.Setenv(core.EnvDiscovered, writeDiscovered(t, time.Now(), "gpt-6-luna", "gpt-6-sol"))
	payload := []string{"from-hook"}
	got := core.ResolveSession("codex", &payload)
	if len(got.IDs) != 1 || got.IDs[0] != "from-hook" {
		t.Fatalf("got %+v", got)
	}
}

func TestExpiredOrDisabledDiscoveryIsUnknown(t *testing.T) {
	t.Setenv(core.EnvDiscovery, "")
	t.Setenv(core.EnvDiscovered, writeDiscovered(t, time.Now().Add(-48*time.Hour), "a"))
	if core.LoadDiscoveredSession("codex", time.Now()).Known {
		t.Error("48h-old entry must expire under the 24h default")
	}
	t.Setenv(core.EnvDiscoveryTTL, "0")
	if !core.LoadDiscoveredSession("codex", time.Now()).Known {
		t.Error("ttl 0 keeps the entry")
	}
	t.Setenv(core.EnvDiscoveryTTL, "")
	t.Setenv(core.EnvDiscovered, writeDiscovered(t, time.Now(), "a"))
	t.Setenv(core.EnvDiscovery, "off")
	if core.LoadDiscoveredSession("codex", time.Now()).Known {
		t.Error("DOWNSHIFT_DISCOVERY=off must disable the layer")
	}
}

func TestDiscoveredMissingHarnessOrFileIsUnknown(t *testing.T) {
	t.Setenv(core.EnvDiscovery, "")
	t.Setenv(core.EnvDiscovered, writeDiscovered(t, time.Now(), "a"))
	if core.LoadDiscoveredSession("grok", time.Now()).Known {
		t.Error("harness without an entry is unknown")
	}
	t.Setenv(core.EnvDiscovered, filepath.Join(t.TempDir(), "absent.json"))
	if core.LoadDiscoveredSession("codex", time.Now()).Known {
		t.Error("missing file is unknown")
	}
}

func TestFutureDatedDiscoveryEntryExpires(t *testing.T) {
	t.Setenv(core.EnvDiscovery, "")
	t.Setenv(core.EnvDiscovered, writeDiscovered(t, time.Now().Add(48*time.Hour), "a"))
	if core.LoadDiscoveredSession("codex", time.Now()).Known {
		t.Error("an entry dated in the future must not live forever")
	}
	t.Setenv(core.EnvDiscovered, writeDiscovered(t, time.Now().Add(time.Minute), "a"))
	if !core.LoadDiscoveredSession("codex", time.Now()).Known {
		t.Error("a small clock skew must be tolerated")
	}
}

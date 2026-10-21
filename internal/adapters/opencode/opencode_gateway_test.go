// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package opencode_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/opencode"
	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/gateway"
	"github.com/tiagovilasboas/downshift/internal/gateway/openrouter"
)

// The OpenRouter adapter must be registered for gateway rows to load, as
// cmd/downshift does at startup.
func init() {
	gateway.Register(openrouter.New(nil))
}

const exactLiveSlug = "deepseek/deepseek-v4.1-flash-1017"

// liveRouteSetup writes an openrouter.json with one small row carrying route,
// and the native opencode small and frontier ids as the session. It returns
// a spawn on the frontier model.
func liveRouteSetup(t *testing.T, route string) (opencode.Request, core.Resolver) {
	t.Helper()
	state := t.TempDir()
	t.Setenv("HOME", state)
	t.Setenv("DOWNSHIFT_STATE_DIR", state)
	body := `{"version":1,"harness":"opencode","models":[{"id":"` + exactLiveSlug + `","tier":"small","max_effort":"medium","route":"` + route + `"}]}`
	if err := os.WriteFile(filepath.Join(state, "openrouter.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	// Load catalog from THIS isolated state (with the gateway file we just wrote),
	// not from tierIDs which would re-isolate to a different TempDir.
	cat := catalog.Load()
	small := cat.ModelFor("opencode", core.TierSmall).ID
	frontier := cat.ModelFor("opencode", core.TierFrontier).ID
	allowlist(t, small, frontier)
	return request(trivialRename, frontier), cat
}

// A live row answers the spawn with the exact OpenCode provider/model pair:
// provider "openrouter", id the allowlisted slug.
func TestHandle_LiveRowEmitsExactGatewayID(t *testing.T) {
	req, res := liveRouteSetup(t, "live")
	resp, _, _ := opencode.Handle(req, res)
	if resp.Action != "route" || resp.Model == nil {
		t.Fatalf("Handle() = %+v, want a route answer", resp)
	}
	if resp.Model.ProviderID != "openrouter" || resp.Model.ID != exactLiveSlug {
		t.Fatalf("model = %s/%s, want openrouter/%s", resp.Model.ProviderID, resp.Model.ID, exactLiveSlug)
	}
	wire, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	want := `"model":{"providerID":"openrouter","id":"` + exactLiveSlug + `"}`
	if !strings.Contains(string(wire), want) {
		t.Errorf("wire = %s, want it to carry %s", wire, want)
	}
}

// A shadow row never changes the emitted model: the answer is native.
func TestHandle_ShadowRowKeepsNativeModel(t *testing.T) {
	req, res := liveRouteSetup(t, "shadow")
	resp, _, _ := opencode.Handle(req, res)
	if resp.Model != nil && (resp.Model.ProviderID == "openrouter" || resp.Model.ID == exactLiveSlug) {
		t.Fatalf("shadow row changed the emitted model: %+v", resp.Model)
	}
}

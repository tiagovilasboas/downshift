// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package opencode_test

import (
	"os"
	"path/filepath"
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
// The requested model arrives in the payload as the child session's model
// object ({id, providerID, variant}), the shape OpenCode 2.0.32 stores in
// session_v2.model. A catalog id with a variant must reach the decision as a
// known baseline, never as an unknown one.
func TestHandle_ChildSessionModelObjectIsTheRequestedModel(t *testing.T) {
	_, res := liveRouteSetup(t, "live")
	payload := `{"v":1,"kind":"route","harness":{"id":"opencode","version":"2.0.32"},
	 "event":"subagent_spawn_intent",
	 "correlation":{"child":"0123456789abcdef0123456789abcdef","parent":"fedcba9876543210"},
	 "agent":"explore","model":{"providerID":"opencode","id":"glm-5.3-flash-free","variant":"max"},
	 "taskText":"rename the userId variable to userIdentifier","tsUtc":"2026-10-21T00:00:00Z"}`
	req, err := opencode.ParseRequest([]byte(payload))
	if err != nil {
		t.Fatal(err)
	}
	if req.RequestedModel() != "glm-5.3-flash-free" {
		t.Fatalf("RequestedModel() = %q", req.RequestedModel())
	}
	_, _, d := opencode.Handle(req, res)
	if d.Verdict == core.VerdictUnknown {
		t.Fatalf("baseline unknown for a catalog model")
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

// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package catalog

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/paths"
)

// useStateDir points the state dir at an empty temp home and returns nothing;
// writeStateFile puts a file under it.
func useStateDir(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
}

func writeStateFile(t *testing.T, name func() (string, error), body string) {
	t.Helper()
	p, err := name()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func allowlist(models string) string {
	return `{"version":1,"harness":"opencode","models":[` + models + `]}`
}

func TestParseOpenRouterAllowlist_Rejects(t *testing.T) {
	cases := map[string]string{
		"invalid JSON":          `{"version":1,`,
		"wrong version":         `{"version":2,"harness":"opencode","models":[]}`,
		"wrong harness":         `{"version":1,"harness":"claude-code","models":[]}`,
		"unknown tier":          allowlist(`{"id":"x-ai/grok-x","tier":"huge","max_effort":"low"}`),
		"missing tier":          allowlist(`{"id":"x-ai/grok-x","max_effort":"low"}`),
		"id without slash":      allowlist(`{"id":"grok-x","tier":"small","max_effort":"low"}`),
		"empty vendor":          allowlist(`{"id":"/grok-x","tier":"small","max_effort":"low"}`),
		"empty model":           allowlist(`{"id":"x-ai/","tier":"small","max_effort":"low"}`),
		"two slashes":           allowlist(`{"id":"a/b/c","tier":"small","max_effort":"low"}`),
		"duplicate ids":         allowlist(`{"id":"x-ai/grok-x","tier":"small","max_effort":"low"},{"id":"x-ai/grok-x","tier":"mid","max_effort":"low"}`),
		"duplicate ids by case": allowlist(`{"id":"X-ai/grok-x","tier":"small","max_effort":"low"},{"id":"x-ai/GROK-X","tier":"mid","max_effort":"low"}`),
		"unknown max_effort":    allowlist(`{"id":"x-ai/grok-x","tier":"small","max_effort":"max"}`),
		"missing max_effort":    allowlist(`{"id":"x-ai/grok-x","tier":"small"}`),
		"one bad row, one good": allowlist(`{"id":"x-ai/grok-x","tier":"small","max_effort":"low"},{"id":"bad","tier":"small","max_effort":"low"}`),
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			rows, err := parseOpenRouterAllowlist([]byte(body))
			if err == nil {
				t.Fatalf("want an error, got rows %+v", rows)
			}
			if rows != nil {
				t.Errorf("a rejected file must return no rows, got %+v", rows)
			}
		})
	}
}

func TestParseOpenRouterAllowlist_Accepts(t *testing.T) {
	rows, err := parseOpenRouterAllowlist([]byte(allowlist(
		`{"id":"deepseek/deepseek-v4.1-flash","tier":"mid","max_effort":"medium","zdr":true},` +
			`{"id":"x-ai/grok-x","tier":"Frontier","max_effort":"high"}`)))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := []Entry{
		{
			ID: "deepseek/deepseek-v4.1-flash", Provider: "openrouter", Harness: "opencode",
			Tier: "mid", Routing: RoutingShadow, Efforts: []string{"low", "medium"}, ZDR: true,
		},
		{
			ID: "x-ai/grok-x", Provider: "openrouter", Harness: "opencode",
			Tier: "frontier", Routing: RoutingShadow, Efforts: []string{"low", "medium", "high"},
		},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows =\n%+v\nwant\n%+v", rows, want)
	}
}

func TestParseOpenRouterAllowlist_EffortCap(t *testing.T) {
	cases := map[string][]string{
		"low":    {"low"},
		"medium": {"low", "medium"},
		"high":   {"low", "medium", "high"},
	}
	for capValue, want := range cases {
		rows, err := parseOpenRouterAllowlist([]byte(allowlist(
			`{"id":"x-ai/grok-x","tier":"small","max_effort":"` + capValue + `"}`)))
		if err != nil {
			t.Fatalf("max_effort %q: %v", capValue, err)
		}
		if !reflect.DeepEqual(rows[0].Efforts, want) {
			t.Errorf("max_effort %q: efforts = %v, want %v", capValue, rows[0].Efforts, want)
		}
	}
}

func TestOpenRouterAllowlist_MissingFileIsSilent(t *testing.T) {
	useStateDir(t)
	var warn strings.Builder
	load(&warn)
	if warn.Len() != 0 {
		t.Errorf("no allowlist must not warn, got %q", warn.String())
	}
}

func TestOpenRouterAllowlist_ValidFileIsShadowOnly(t *testing.T) {
	useStateDir(t)
	writeStateFile(t, paths.GatewayAllowlistPath, allowlist(
		`{"id":"x-ai/grok-x","tier":"small","max_effort":"low","zdr":true}`))

	var warn strings.Builder
	c := load(&warn)
	if warn.Len() != 0 {
		t.Fatalf("valid allowlist warned: %q", warn.String())
	}
	if got := c.ModelFor("opencode", core.TierSmall); got.ID == "x-ai/grok-x" {
		t.Errorf("ModelFor small = %q: an allowlisted row must not be automatic", got.ID)
	}
	if !c.IsExplicitOnly("opencode", "x-ai/grok-x") {
		t.Error("an allowlisted row must not be writable as a current model")
	}
	if shadow := c.ShadowGateways(); len(shadow) != 1 || shadow[0].ID != "x-ai/grok-x" || shadow[0].Tier != core.TierSmall {
		t.Errorf("ShadowGateways = %+v, want the allowlisted row", shadow)
	}
	e, ok := c.EntryFor("opencode", "x-ai/grok-x")
	if !ok || !e.ZDR {
		t.Errorf("entry = %+v ok=%v, want ZDR", e, ok)
	}
}

func TestOpenRouterAllowlist_BadFileLeavesCatalogUnchanged(t *testing.T) {
	useStateDir(t)
	baseline := load(&strings.Builder{})

	bad := map[string]string{
		"invalid JSON":  `{"version":1,`,
		"unknown tier":  allowlist(`{"id":"x-ai/grok-x","tier":"huge","max_effort":"low"}`),
		"id no slash":   allowlist(`{"id":"grok-x","tier":"small","max_effort":"low"}`),
		"duplicate ids": allowlist(`{"id":"x-ai/grok-x","tier":"small","max_effort":"low"},{"id":"x-ai/grok-x","tier":"small","max_effort":"low"}`),
		"one bad row":   allowlist(`{"id":"x-ai/grok-x","tier":"small","max_effort":"low"},{"id":"bad","tier":"small","max_effort":"low"}`),
		"over 256 KiB":  strings.Repeat(" ", maxOpenRouterAllowlistBytes+1),
	}
	for name, body := range bad {
		t.Run(name, func(t *testing.T) {
			writeStateFile(t, paths.GatewayAllowlistPath, body)
			var warn strings.Builder
			c := load(&warn)
			if !strings.Contains(warn.String(), "ignoring it") {
				t.Errorf("want a warning, got %q", warn.String())
			}
			if len(c.Entries()) != len(baseline.Entries()) {
				t.Errorf("entries = %d, want the %d embedded ones", len(c.Entries()), len(baseline.Entries()))
			}
			for _, e := range c.Entries() {
				if e.Provider == "openrouter" {
					t.Errorf("a rejected file added gateway row %+v", e)
				}
			}
			for _, tier := range []core.Tier{core.TierSmall, core.TierMid, core.TierFrontier} {
				if got, want := c.ModelFor("opencode", tier), baseline.ModelFor("opencode", tier); got != want {
					t.Errorf("ModelFor(%v) = %+v, want %+v", tier, got, want)
				}
			}
		})
	}
}

func TestOpenRouterAllowlist_IdClashingWithOverrideIsRejected(t *testing.T) {
	useStateDir(t)
	writeStateFile(t, paths.CatalogPath, `{"entries":[
		{"id":"z-ai/glm","harness":"opencode","provider":"openrouter","tier":"mid"}]}`)
	writeStateFile(t, paths.GatewayAllowlistPath, allowlist(
		`{"id":"z-ai/glm","tier":"small","max_effort":"low"}`))

	var warn strings.Builder
	c := load(&warn)
	if !strings.Contains(warn.String(), "ignoring it") {
		t.Fatalf("want a merge warning, got %q", warn.String())
	}
	if !c.IsExplicitOnly("opencode", "z-ai/glm") {
		t.Error("the override row must keep its explicit_only default after a rejected allowlist")
	}
}

// A row that is not allowlisted is never an automatic target, even when a
// user catalog names it with a tier.
func TestOpenRouterRow_NotAllowlistedIsNeverAutomatic(t *testing.T) {
	useStateDir(t)
	writeStateFile(t, paths.CatalogPath, `{"entries":[
		{"id":"z-ai/glm","harness":"opencode","provider":"openrouter","tier":"mid"}]}`)
	writeStateFile(t, paths.GatewayAllowlistPath, allowlist(
		`{"id":"x-ai/grok-x","tier":"small","max_effort":"low"}`))

	c := load(&strings.Builder{})
	for _, tier := range []core.Tier{core.TierSmall, core.TierMid, core.TierFrontier} {
		if got := c.ModelFor("opencode", tier); got.ID == "z-ai/glm" {
			t.Errorf("ModelFor(%v) returned the non-allowlisted gateway row", tier)
		}
	}
	if !c.IsExplicitOnly("opencode", "z-ai/glm") {
		t.Error("a gateway row without routing must be explicit_only")
	}
}

func TestGatewayRows_ExplicitOnlyUnlessTheRowSaysOtherwise(t *testing.T) {
	cases := []struct {
		name string
		row  string
		id   string
		want bool // IsExplicitOnly
	}{
		{"openrouter without routing", `{"id":"a/b","harness":"opencode","provider":"openrouter","tier":"mid"}`, "a/b", true},
		{"openrouter automatic", `{"id":"a/b","harness":"opencode","provider":"openrouter","tier":"mid","routing":"automatic"}`, "a/b", false},
		{"native without routing", `{"id":"grok-x","harness":"opencode","provider":"opencode","tier":"small"}`, "grok-x", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := parse([]byte(`{"entries":[` + tc.row + `]}`))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got := c.IsExplicitOnly("opencode", tc.id); got != tc.want {
				t.Errorf("IsExplicitOnly = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestEmbeddedCatalog_HasNoGatewayRows(t *testing.T) {
	f, err := decode(embeddedJSON)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, e := range f.Entries {
		if e.Provider == "openrouter" {
			t.Errorf("embedded catalog ships gateway row %q; gateway rows come from the user allowlist", e.ID)
		}
	}
}

// A user override written before prices were dropped still carries the old
// price keys (input_cost_per_1m and the rest) and a top-level prices_as_of.
// The decoder is not strict, so those keys are ignored and the override still
// loads and routes: a user is never broken by the field removal.
func TestLoad_OldUserCatalogWithPriceKeysStillLoads(t *testing.T) {
	useStateDir(t)
	writeStateFile(t, paths.CatalogPath, `{
		"prices_as_of": "2026-10-01",
		"entries": [
			{
				"id": "claude-opus-5-5",
				"harness": "claude-code",
				"tier": "frontier",
				"input_cost_per_1m": 4.0,
				"output_cost_per_1m": 20.0,
				"cache_read_cost_per_1m": 0.2,
				"cache_write_cost_per_1m": 5.0
			}
		]
	}`)

	var warn strings.Builder
	c := load(&warn)
	if warn.Len() != 0 {
		t.Fatalf("an old-format override must load without a warning, got %q", warn.String())
	}
	m, ok := c.LookupByID("claude-code", "claude-opus-5-5")
	if !ok || m.ID != "claude-opus-5-5" || m.Tier != core.TierFrontier {
		t.Fatalf("override entry = %+v ok=%v, want claude-opus-5-5 at the frontier tier", m, ok)
	}
}

// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"reflect"
	"testing"
	"time"
)

// withEnv swaps the live seam for a map and restores it afterwards. No
// Config is set, so the accessors read through the seam.
func withEnv(t *testing.T, m map[string]string) {
	t.Helper()
	prev := lookupEnv
	lookupEnv = func(k string) string { return m[k] }
	t.Cleanup(func() { lookupEnv = prev })
}

func TestEnvAccessorsReadTheSeam(t *testing.T) {
	// Empty map: every accessor reports its default.
	withEnv(t, map[string]string{})
	ttl, invalid := discoveryTTL()
	if sessionModelsEnv() != "" || quotaModeRequired() ||
		discoveryOff() || discoveredEnv() != "" ||
		ttl != defaultDiscoveryTTL || invalid || nbDownshiftEnv() ||
		NBDownshiftEnabled() ||
		nativeSessionPathEnv("DOWNSHIFT_CURSOR_NATIVE_FILE") != "" {
		t.Fatal("defaults must be empty/false with an empty environment")
	}

	cases := []struct {
		name string
		key  string
		val  string
		get  func() any
		want any
	}{
		{"session models", EnvSessionModels, "/x/sm.json", func() any { return sessionModelsEnv() }, "/x/sm.json"},
		{"quota mode", EnvQuotaMode, "required", func() any { return quotaModeRequired() }, true},
		{"discovery", EnvDiscovery, "off", func() any { return discoveryOff() }, true},
		{"discovered", EnvDiscovered, "/x/d.json", func() any { return discoveredEnv() }, "/x/d.json"},
		{"discovery ttl", EnvDiscoveryTTL, "1h", func() any { d, _ := discoveryTTL(); return d }, time.Hour},
		{"discovery ttl forever", EnvDiscoveryTTL, "0", func() any { d, inv := discoveryTTL(); return d == 0 && !inv }, true},
		{"discovery ttl invalid", EnvDiscoveryTTL, "soon", func() any { _, inv := discoveryTTL(); return inv }, true},
		{"discovery ttl negative", EnvDiscoveryTTL, "-1h", func() any { _, inv := discoveryTTL(); return inv }, true},
		{"nb downshift", NBDownshiftEnv, "1", func() any { return NBDownshiftEnabled() }, true},
		{"native session", "DOWNSHIFT_CURSOR_NATIVE_FILE", "/x/n.json", func() any { return nativeSessionPathEnv("DOWNSHIFT_CURSOR_NATIVE_FILE") }, "/x/n.json"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			withEnv(t, map[string]string{c.key: c.val})
			if got := c.get(); got != c.want {
				t.Fatalf("%s=%q: got %v, want %v", c.key, c.val, got, c.want)
			}
		})
	}
}

func TestEnvAccessorsKeepExactMatching(t *testing.T) {
	withEnv(t, map[string]string{
		EnvQuotaMode:   "REQUIRED",
		EnvDiscovery:   "0",
		NBDownshiftEnv: "on",
	})
	if quotaModeRequired() || discoveryOff() || NBDownshiftEnabled() {
		t.Fatal("only the documented literal values may switch a variable")
	}
}

// TestSetConfigPinsTheSnapshot: once a Config is set, the accessors read it
// and ignore the environment until it is restored.
func TestSetConfigPinsTheSnapshot(t *testing.T) {
	withEnv(t, map[string]string{NBDownshiftEnv: "1", EnvSessionModels: "/env/sm.json"})
	if !NBDownshiftEnabled() || sessionModelsEnv() != "/env/sm.json" {
		t.Fatal("precondition: live environment is read when no Config is set")
	}
	restore := SetConfig(Config{SessionModelsPath: "/pinned/sm.json",
		NativeSessionPaths: map[string]string{"DOWNSHIFT_CURSOR_NATIVE_FILE": "/pinned/n.json"}})
	if NBDownshiftEnabled() || sessionModelsEnv() != "/pinned/sm.json" ||
		nativeSessionPathEnv("DOWNSHIFT_CURSOR_NATIVE_FILE") != "/pinned/n.json" {
		t.Fatal("accessors must read the pinned Config, not the environment")
	}
	restore()
	if !NBDownshiftEnabled() || sessionModelsEnv() != "/env/sm.json" {
		t.Fatal("restore must return to the live environment")
	}
}

// TestConfigFromEnvMatchesAccessors: the typed snapshot and the live path
// agree on every variable, including the native session paths taken from
// the capability table.
func TestConfigFromEnvMatchesAccessors(t *testing.T) {
	m := map[string]string{
		EnvSessionModels: "/x/sm.json", EnvQuotaMode: "required",
		EnvDiscovery: "off", EnvDiscovered: "/x/d.json",
		EnvDiscoveryTTL: "90m", NBDownshiftEnv: "1",
	}
	for _, n := range AllHarnessCaps() {
		if n.Caps.NativeSessionEnv != "" {
			m[n.Caps.NativeSessionEnv] = "/x/" + n.Harness + ".json"
		}
	}
	got := ConfigFromEnv(func(k string) string { return m[k] })
	want := Config{
		SessionModelsPath: "/x/sm.json", QuotaRequired: true,
		DiscoveryOff: true, DiscoveredPath: "/x/d.json",
		DiscoveryTTL: 90 * time.Minute, NBDownshift: true,
	}
	paths := got.NativeSessionPaths
	got.NativeSessionPaths = nil
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ConfigFromEnv = %+v, want %+v", got, want)
	}
	for _, n := range AllHarnessCaps() {
		if env := n.Caps.NativeSessionEnv; env != "" && paths[env] != m[env] {
			t.Fatalf("native path %s = %q, want %q", env, paths[env], m[env])
		}
	}
	if ConfigFromEnv(func(string) string { return "" }).NativeSessionPaths["DOWNSHIFT_CURSOR_NATIVE_FILE"] != "" {
		t.Fatal("unset native path must be empty")
	}
}

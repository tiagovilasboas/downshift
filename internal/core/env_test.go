// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import "testing"

// withEnv swaps the package seam for a map and restores it afterwards.
func withEnv(t *testing.T, m map[string]string) {
	t.Helper()
	prev := lookupEnv
	lookupEnv = func(k string) string { return m[k] }
	t.Cleanup(func() { lookupEnv = prev })
}

func TestEnvAccessorsReadTheSeam(t *testing.T) {
	// Empty map: every accessor reports its default.
	withEnv(t, map[string]string{})
	if sessionModelsEnv() != "" || quotaModeRequired() || explicitUpshiftEnv() ||
		inferSessionOff() || discoveryOff() || discoveredEnv() != "" ||
		discoveryTTLEnv() != "" || nbUpshiftEnv() || nbDownshiftEnv() ||
		scoreRoutingEnv() || NBUpshiftEnabled() || NBDownshiftEnabled() ||
		ScoreRoutingEnabled() ||
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
		{"explicit upshift", EnvExplicitUpshift, "1", func() any { return explicitUpshiftEnv() }, true},
		{"infer session", EnvInferSession, "off", func() any { return inferSessionOff() }, true},
		{"discovery", EnvDiscovery, "off", func() any { return discoveryOff() }, true},
		{"discovered", EnvDiscovered, "/x/d.json", func() any { return discoveredEnv() }, "/x/d.json"},
		{"discovery ttl", EnvDiscoveryTTL, "1h", func() any { return discoveryTTLEnv() }, "1h"},
		{"nb upshift", NBUpshiftEnv, "1", func() any { return NBUpshiftEnabled() }, true},
		{"nb downshift", NBDownshiftEnv, "1", func() any { return NBDownshiftEnabled() }, true},
		{"score routing", ScoreRoutingEnv, "1", func() any { return ScoreRoutingEnabled() }, true},
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
		EnvQuotaMode:       "REQUIRED",
		EnvExplicitUpshift: "true",
		EnvInferSession:    "OFF",
		EnvDiscovery:       "0",
		NBUpshiftEnv:       "yes",
		NBDownshiftEnv:     "on",
		ScoreRoutingEnv:    "true",
	})
	if quotaModeRequired() || explicitUpshiftEnv() || inferSessionOff() || discoveryOff() ||
		NBUpshiftEnabled() || NBDownshiftEnabled() || ScoreRoutingEnabled() {
		t.Fatal("only the documented literal values may switch a variable")
	}
}

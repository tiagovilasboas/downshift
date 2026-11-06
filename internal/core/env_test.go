// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"reflect"
	"testing"
	"time"
)

// withEnv swaps the live seam for a map and restores it afterwards. No
// Config is set, so nativeSessionPathEnv reads through the seam.
func withEnv(t *testing.T, m map[string]string) {
	t.Helper()
	prev := lookupEnv
	lookupEnv = func(k string) string { return m[k] }
	t.Cleanup(func() { lookupEnv = prev })
}

func TestNativeSessionPathReadsTheSeam(t *testing.T) {
	withEnv(t, map[string]string{})
	if nativeSessionPathEnv("DOWNSHIFT_CURSOR_NATIVE_FILE") != "" || nativeSessionPathEnv("") != "" {
		t.Fatal("an unset native path must be empty")
	}
	withEnv(t, map[string]string{"DOWNSHIFT_CURSOR_NATIVE_FILE": "/x/n.json"})
	if got := nativeSessionPathEnv("DOWNSHIFT_CURSOR_NATIVE_FILE"); got != "/x/n.json" {
		t.Fatalf("native path = %q, want /x/n.json", got)
	}
}

func TestConfigFromEnvKeepsExactMatching(t *testing.T) {
	c := ConfigFromEnv(func(k string) string {
		return map[string]string{EnvQuotaMode: "REQUIRED", EnvDiscovery: "0"}[k]
	})
	if c.QuotaRequired || c.DiscoveryOff {
		t.Fatal("only the documented literal values may switch a variable")
	}
}

// TestSetConfigPinsTheSnapshot: once a Config is set, the native session path
// is read from it and the environment is ignored until it is restored.
func TestSetConfigPinsTheSnapshot(t *testing.T) {
	withEnv(t, map[string]string{"DOWNSHIFT_CURSOR_NATIVE_FILE": "/env/n.json"})
	restore := SetConfig(Config{NativeSessionPaths: map[string]string{"DOWNSHIFT_CURSOR_NATIVE_FILE": "/pinned/n.json"}})
	if got := nativeSessionPathEnv("DOWNSHIFT_CURSOR_NATIVE_FILE"); got != "/pinned/n.json" {
		t.Fatalf("pinned native path = %q, want /pinned/n.json", got)
	}
	restore()
	if got := nativeSessionPathEnv("DOWNSHIFT_CURSOR_NATIVE_FILE"); got != "/env/n.json" {
		t.Fatalf("restore must return to the live environment, got %q", got)
	}
}

// TestConfigFromEnvReadsEveryVariable: the typed snapshot carries every
// variable, including the native session paths taken from the capability
// table.
func TestConfigFromEnvReadsEveryVariable(t *testing.T) {
	m := map[string]string{
		EnvSessionModels: "/x/sm.json", EnvQuotaMode: "required",
		EnvDiscovery: "off", EnvDiscovered: "/x/d.json",
		EnvDiscoveryTTL: "90m",
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
		DiscoveryTTL: 90 * time.Minute,
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

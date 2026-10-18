// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"reflect"
	"testing"
	"time"
)

func TestCapsForMatchesAllHarnessCaps(t *testing.T) {
	for _, n := range AllHarnessCaps() {
		got, ok := CapsFor(n.Harness)
		if !ok || !reflect.DeepEqual(got, n.Caps) {
			t.Fatalf("CapsFor(%q) = %+v, %v", n.Harness, got, ok)
		}
	}
	if _, ok := CapsFor("nope"); ok {
		t.Fatal("unknown harness found")
	}
}

func TestClaudeCodeCapsDeclareBothHookEntries(t *testing.T) {
	want := []string{"settings-hook", "mod"}
	if !reflect.DeepEqual(ClaudeCodeCaps.HookEntries, want) {
		t.Fatalf("HookEntries = %v, want %v", ClaudeCodeCaps.HookEntries, want)
	}
	for _, n := range AllHarnessCaps() {
		if n.Harness == "claude-code" {
			continue
		}
		if len(n.Caps.HookEntries) > 1 {
			t.Fatalf("%s unexpectedly declares multiple HookEntries: %v", n.Harness, n.Caps.HookEntries)
		}
	}
}

// A new harness declaring the capability needs no edit to shared code.
func TestHypotheticalHarnessNativeSession(t *testing.T) {
	now := time.Now().UTC()
	t.Setenv("DOWNSHIFT_FAKE_NATIVE_FILE", writeNativeFixture(t, nativeFixture(now)))
	caps := HarnessCapabilities{NativeSessionEnv: "DOWNSHIFT_FAKE_NATIVE_FILE", NativeUsageSource: "cursor-usage"}
	s, ok := loadNativeSession(caps, now)
	if !ok || !s.Known || !s.NativeAvailability || len(s.IDs) != 4 {
		t.Fatalf("hypothetical harness not handled: %+v %v", s, ok)
	}
	if _, ok := loadNativeSession(HarnessCapabilities{}, now); ok {
		t.Fatal("harness without the capability must not load")
	}
}

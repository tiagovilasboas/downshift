// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"io"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/hookport"
)

// A registry entry alone gives a new harness hook, port and try support.
func TestRegistryEntryGivesHookPortAndTry(t *testing.T) {
	const id = "fakeharness"
	hookRan := false
	harnessRegistry[id] = harnessEntry{
		Hook: func(io.Reader, core.Resolver) int { hookRan = true; return 0 },
		Port: portPtr(hookport.Port{ID: id}),
		Try: func(prompt, current string, _ core.Resolver) tryResult {
			return tryResult{written: "fake-model"}
		},
	}
	t.Cleanup(func() { delete(harnessRegistry, id) })

	if h := harnessRegistry[id]; h.Hook == nil || h.Hook(nil, nil) != 0 || !hookRan {
		t.Fatal("hook not dispatched from registry")
	}
	if portFor(id).ID != id {
		t.Fatal("port not read from registry")
	}
	res, known := tryHook(id, "p", "", catalog.Load())
	if !known || res.written != "fake-model" {
		t.Fatalf("tryHook = %+v known=%t", res, known)
	}
	found := false
	for _, n := range validTryHarnesses(catalog.Load()) {
		found = found || n == id
	}
	if !found {
		t.Fatal("validTryHarnesses missing registry harness")
	}
}

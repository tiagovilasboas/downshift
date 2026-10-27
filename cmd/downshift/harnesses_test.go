// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"io"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/hookport"
)

// A registry entry alone gives a new harness hook and port.
func TestRegistryEntryGivesHookAndPort(t *testing.T) {
	const id = "fakeharness"
	hookRan := false
	harnessRegistry[id] = harnessEntry{
		Hook: func(io.Reader, core.Resolver) int { hookRan = true; return 0 },
		Port: portPtr(hookport.Port{ID: id}),
	}
	t.Cleanup(func() { delete(harnessRegistry, id) })

	if h := harnessRegistry[id]; h.Hook == nil || h.Hook(nil, nil) != 0 || !hookRan {
		t.Fatal("hook not dispatched from registry")
	}
	if portFor(id).ID != id {
		t.Fatal("port not read from registry")
	}
}

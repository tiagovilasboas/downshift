// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"path/filepath"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/session"
	"github.com/tiagovilasboas/downshift/internal/testenv"
)

// TestHookRunnerReadsConfigOnce proves the shared runner is the one place
// the hook configuration is read: the value core sees inside the handler is
// the runner's snapshot (taken through hookGetenv), not the live process
// environment, and the snapshot is released when the runner returns. The
// discovery TTL is the probe: the process leaves it at its default, the
// runner's getenv sets it to 90 minutes.
func TestHookRunnerReadsConfigOnce(t *testing.T) {
	testenv.SetHome(t, t.TempDir())
	t.Setenv("DOWNSHIFT_EVENT_LOG", filepath.Join(t.TempDir(), "events.jsonl"))
	t.Setenv(core.EnvDiscoveryTTL, "")
	const snapshot = 90 * time.Minute
	prev := hookGetenv
	hookGetenv = func(k string) string {
		if k == core.EnvDiscoveryTTL {
			return snapshot.String()
		}
		return ""
	}
	t.Cleanup(func() { hookGetenv = prev })

	if ttl, _ := session.DiscoveryTTL(); ttl == snapshot {
		t.Fatal("precondition: the process environment leaves the discovery TTL at its default")
	}
	var seen time.Duration
	captureStdout(func() {
		runHookAdapter(
			"test",
			bytes.NewReader([]byte(`{}`)),
			func([]byte) (struct{}, error) { return struct{}{}, nil },
			func(struct{}) (any, string, core.Decision) {
				seen, _ = session.DiscoveryTTL()
				return nil, "", core.Decision{}
			},
			printAllow,
		)
	})
	if seen != snapshot {
		t.Fatalf("inside the runner core must read the runner's configuration snapshot: discovery TTL %v, want %v", seen, snapshot)
	}
	if ttl, _ := session.DiscoveryTTL(); ttl == snapshot {
		t.Fatal("the runner must restore the configuration on return")
	}
}

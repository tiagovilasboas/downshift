// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// TestHookRunnerReadsConfigOnce proves the shared runner is the one place
// the hook configuration is read: the value core sees inside the handler is
// the runner's snapshot (taken through hookGetenv), not the live process
// environment, and the snapshot is released when the runner returns.
func TestHookRunnerReadsConfigOnce(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DOWNSHIFT_EVENT_LOG", filepath.Join(t.TempDir(), "events.jsonl"))
	t.Setenv(core.NBUpshiftEnv, "")
	prev := hookGetenv
	hookGetenv = func(k string) string {
		if k == core.NBUpshiftEnv {
			return "1"
		}
		return ""
	}
	t.Cleanup(func() { hookGetenv = prev })

	if core.NBUpshiftEnabled() {
		t.Fatal("precondition: the process environment leaves NB upshift off")
	}
	seen := false
	captureStdout(func() {
		runHookAdapter(
			"test",
			bytes.NewReader([]byte(`{}`)),
			func([]byte) (struct{}, error) { return struct{}{}, nil },
			func(struct{}) (any, string, core.Decision) {
				seen = core.NBUpshiftEnabled()
				return nil, "", core.Decision{}
			},
			printAllow,
		)
	})
	if !seen {
		t.Fatal("inside the runner core must read the runner's configuration snapshot")
	}
	if core.NBUpshiftEnabled() {
		t.Fatal("the runner must restore the configuration on return")
	}
}

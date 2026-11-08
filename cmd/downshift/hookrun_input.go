// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/paths"
)

// foreignPayloadOwner names the harness that sent a payload to the hook of
// harness when it is not that harness's own (core.ForeignPayloadOwner on the
// declared PayloadMarkers), or "". Only top-level keys are read; a payload that
// is not a JSON object is left to the normal parse path.
func foreignPayloadOwner(harness string, data []byte) string {
	var top map[string]json.RawMessage
	if json.Unmarshal(data, &top) != nil {
		return ""
	}
	return core.ForeignPayloadOwner(harness, func(key string) bool {
		_, ok := top[key]
		return ok
	})
}

func foreignPayloadNote(harness, owner string) string {
	return fmt.Sprintf("downshift: %s payload reached the %s hook; left to the %s hook, nothing recorded\n", owner, harness, owner)
}

// warnSessionUnknown prints one stderr line per hook call when the adapter
// could not resolve a session allowlist: without it the hook never rewrites,
// which otherwise looks like a silent no-op install.
func warnSessionUnknown(d core.Decision) {
	if d.Harness == "" || !d.Session.Unknown {
		return
	}
	fmt.Fprintf(os.Stderr, "downshift: no session allowlist for %s, so no model is rewritten. "+
		"Add \"%s\": [<model ids>] to %s (see docs/reference/session-models.md)\n",
		d.Harness, d.Harness, sessionModelsPathForDisplay())
}

func sessionModelsPathForDisplay() string {
	return paths.DisplaySessionModels()
}

// readHookPayload prevents an unresponsive hook stdin from blocking a spawn.
// The reader goroutine may remain blocked until process exit, but the hook
// returns its fail-open response at the deadline and the short-lived process
// is then reclaimed by the harness OS process lifecycle.
func readHookPayload(in io.Reader, limit int64) ([]byte, error) {
	type result struct {
		data []byte
		err  error
	}
	done := make(chan result, 1)
	go func() {
		data, err := io.ReadAll(io.LimitReader(in, limit))
		done <- result{data: data, err: err}
	}()
	select {
	case result := <-done:
		return result.data, result.err
	case <-time.After(hookReadTimeout):
		return nil, errHookReadTimeout
	}
}

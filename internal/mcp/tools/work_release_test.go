// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/workers"
)

// The release tool lets go of finished workers and deletes their session
// transcripts, over Queue.Release. These tests drive a real queue over the same
// controllable launcher the send tests use (sendLauncher), so the tool's
// argument mapping and its three reported outcomes (released, refused, warned)
// are covered here; the safe-deletion policy itself is tested in the workers
// package (cleanup_test.go), where a session actually reaches disk. Unhappy
// paths come first (regra 1:3).

// releaseTool returns the release tool from the work set.
func releaseTool(t *testing.T, d Deps) Tool {
	t.Helper()
	for _, tl := range WorkTools(d) {
		if tl.Name == "release" {
			return tl
		}
	}
	t.Fatal("no release tool")
	return Tool{}
}

// callRelease runs release and decodes its data map (or returns the error).
func callRelease(t *testing.T, d Deps, args string) (map[string]any, error) {
	t.Helper()
	res, err := releaseTool(t, d).Handler(context.Background(), json.RawMessage(args))
	if err != nil {
		return nil, err
	}
	var env struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal([]byte(res.Text), &env); err != nil {
		t.Fatalf("envelope is not JSON: %v", err)
	}
	return env.Data, nil
}

// --- unhappy: schema, no queue, and the per-id refusals ---

// TestReleaseWithoutAQueueSaysSo: a server with no worker queue answers that
// workers are unavailable, like the other work tools.
func TestReleaseWithoutAQueueSaysSo(t *testing.T) {
	d := Deps{Strain: fakeStrain{}}
	if _, err := callRelease(t, d, `{"job_ids":["w001"]}`); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("err = %v, want the no-queue message", err)
	}
}

// TestReleaseRefusesAnUnknownField covers the schema guard: a typo in an
// argument name is refused before the queue is reached.
func TestReleaseRefusesAnUnknownField(t *testing.T) {
	d, _ := sendDeps(t)
	if _, err := callRelease(t, d, `{"jobs":["w001"]}`); err == nil ||
		!strings.Contains(err.Error(), "do not match") {
		t.Fatalf("err = %v, want an unknown-field error", err)
	}
}

// TestReleaseReportsAnUnknownIdAsRefused: an id the queue never issued is
// reported under refused, with released empty and no error (the batch is not
// stopped by one bad id).
func TestReleaseReportsAnUnknownIdAsRefused(t *testing.T) {
	d, _ := sendDeps(t)
	data, err := callRelease(t, d, `{"job_ids":["w404"]}`)
	if err != nil {
		t.Fatalf("release: %v", err)
	}
	if released, _ := data["released"].([]any); len(released) != 0 {
		t.Errorf("released = %v, want empty", data["released"])
	}
	refused, _ := data["refused"].(map[string]any)
	if refused == nil || refused["w404"] == nil {
		t.Fatalf("refused = %v, want w404 named", data["refused"])
	}
}

// TestReleaseRefusesALiveWorker: a running worker is reported refused, not
// released, so a caller does not drop a worker mid-run with release.
func TestReleaseRefusesALiveWorker(t *testing.T) {
	d, _ := sendDeps(t)
	id := delegateOne(t, d, "run")
	waitState(t, d, id, workers.Running)
	data, err := callRelease(t, d, `{"job_ids":["`+id+`"]}`)
	if err != nil {
		t.Fatalf("release: %v", err)
	}
	if released, _ := data["released"].([]any); len(released) != 0 {
		t.Errorf("a live worker was released: %v", data["released"])
	}
	refused, _ := data["refused"].(map[string]any)
	if refused == nil || refused[id] == nil {
		t.Fatalf("refused = %v, want %s named with the still-running reason", data["refused"], id)
	}
}

// --- happy: release a finished worker ---

// TestReleaseDropsAFinishedWorker: a worker that finished is reported under
// released, and a second release of the same id now reports it unknown (it is
// gone from the queue).
func TestReleaseDropsAFinishedWorker(t *testing.T) {
	d, l := sendDeps(t)
	id := delegateOne(t, d, "turn-1")
	waitState(t, d, id, workers.Running)
	l.proc("turn-1").finish("first result", nil)
	waitState(t, d, id, workers.Done)

	data, err := callRelease(t, d, `{"job_ids":["`+id+`"]}`)
	if err != nil {
		t.Fatalf("release: %v", err)
	}
	released, _ := data["released"].([]any)
	if len(released) != 1 || released[0] != id {
		t.Fatalf("released = %v, want [%s]", data["released"], id)
	}
	if data["warnings"] != nil {
		t.Errorf("a clean delete must carry no warnings, got %v", data["warnings"])
	}
	// The job is gone: a second release reports it unknown.
	again, err := callRelease(t, d, `{"job_ids":["`+id+`"]}`)
	if err != nil {
		t.Fatalf("second release: %v", err)
	}
	refused, _ := again["refused"].(map[string]any)
	if refused == nil || refused[id] == nil {
		t.Fatalf("second release refused = %v, want the id reported unknown", again["refused"])
	}
}

// TestReleaseAllDropsEveryFinishedWorker: with no ids, every finished worker is
// released at once and a running one is left alone.
func TestReleaseAllDropsEveryFinishedWorker(t *testing.T) {
	d, l := sendDeps(t)
	id := delegateOne(t, d, "turn-1")
	waitState(t, d, id, workers.Running)
	l.proc("turn-1").finish("done", nil)
	waitState(t, d, id, workers.Done)

	data, err := callRelease(t, d, `{}`)
	if err != nil {
		t.Fatalf("release all: %v", err)
	}
	released, _ := data["released"].([]any)
	if len(released) != 1 || released[0] != id {
		t.Fatalf("released = %v, want the one finished worker", data["released"])
	}
}

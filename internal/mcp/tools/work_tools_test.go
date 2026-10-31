// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"encoding/json"
	"strings"
	"testing"
)

// Write is the owner's switch: off, a write request is refused before anything
// starts, and read is always allowed.
func TestWantsWriteRefusesWriteUnlessTheOwnerEnablesIt(t *testing.T) {
	if w, err := wantsWrite("read", false); err != nil || w {
		t.Fatalf("read = %v, %v", w, err)
	}
	if _, err := wantsWrite("write", false); err == nil {
		t.Fatal("write must be refused while the owner has not enabled it")
	}
	if w, err := wantsWrite("write", true); err != nil || !w {
		t.Fatalf("write when enabled = %v, %v", w, err)
	}
	if _, err := wantsWrite("delete", true); err == nil {
		t.Fatal("an unknown mode must be refused")
	}
}

// workerDir defaults a missing dir to the server's resolved project root, keeps
// an explicit dir, and refuses a missing dir when the root is broad so a worker
// never starts in a directory a containment check cannot protect.
func TestWorkerDirDefaultsToRootUnlessBroad(t *testing.T) {
	d := Deps{Root: "/proj"}
	if got, err := d.workerDir(""); err != nil || got != "/proj" {
		t.Fatalf("empty dir = %q, %v; want the root", got, err)
	}
	if got, err := d.workerDir("  "); err != nil || got != "/proj" {
		t.Fatalf("blank dir = %q, %v; want the root", got, err)
	}
	if got, err := d.workerDir("/other"); err != nil || got != "/other" {
		t.Fatalf("explicit dir = %q, %v; want it kept", got, err)
	}

	broad := Deps{Root: "/", Broad: true}
	if _, err := broad.workerDir(""); err == nil {
		t.Fatal("a missing dir must be refused when the root is broad")
	}
	if got, err := broad.workerDir("/proj"); err != nil || got != "/proj" {
		t.Fatalf("explicit dir under a broad root = %q, %v; want it kept", got, err)
	}

	none := Deps{}
	if _, err := none.workerDir(""); err == nil {
		t.Fatal("a missing dir must be refused when no root was resolved")
	}
}

// The collect schema tells the client the timeout cap, so a request for more
// is not silently clamped behind its back.
func TestCollectSchemaStatesTheTimeoutCap(t *testing.T) {
	var collect Tool
	for _, tl := range WorkTools(Deps{}) {
		if tl.Name == "collect" {
			collect = tl
		}
	}
	if collect.Name == "" {
		t.Fatal("no collect tool")
	}
	var schema struct {
		Properties struct {
			TimeoutSeconds struct {
				Maximum int `json:"maximum"`
			} `json:"timeout_seconds"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(collect.InputSchema, &schema); err != nil {
		t.Fatalf("collect schema is not JSON: %v", err)
	}
	if schema.Properties.TimeoutSeconds.Maximum != maxCollectSeconds {
		t.Fatalf("timeout_seconds maximum = %d, want %d", schema.Properties.TimeoutSeconds.Maximum, maxCollectSeconds)
	}
	if !strings.Contains(collectDesc, "45") {
		t.Fatal("the collect description must state the 45-second cap")
	}
}

func TestWorkToolsAreValidAndNamed(t *testing.T) {
	ts := WorkTools(Deps{})
	if err := Validate(ts); err != nil {
		t.Fatal(err)
	}
	titles := map[string]string{
		"delegate": "Start delegated workers",
		"collect":  "Read worker results",
		"cancel":   "Stop workers",
	}
	for _, tl := range ts {
		if want, ok := titles[tl.Name]; !ok || tl.Title != want {
			t.Errorf("%s: Title = %q, want %q", tl.Name, tl.Title, want)
		}
	}
}

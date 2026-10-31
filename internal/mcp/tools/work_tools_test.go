// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/workers"
)

// stubProc is a worker that finishes at once, so a delegate test never touches
// a real CLI or a git worktree. It records the dir it was launched in.
type stubProc struct{}

func (stubProc) Wait() error             { return nil }
func (stubProc) Kill() error             { return nil }
func (stubProc) Report() string          { return "done" }
func (stubProc) Stderr() string          { return "" }
func (stubProc) Tokens() *workers.Tokens { return nil }

// stubLauncher records the spec of every launch and returns a finished stub.
type stubLauncher struct{ specs []workers.Spec }

func (l *stubLauncher) Launch(s workers.Spec) (workers.Process, error) {
	l.specs = append(l.specs, s)
	return stubProc{}, nil
}

// delegateDeps wires a real queue over a stub launcher, with write enabled, so
// delegate exercises the whole write path except the real worker.
func delegateDeps(root string, broad bool, broadDir func(string) bool) (Deps, *stubLauncher) {
	l := &stubLauncher{}
	d := Deps{
		Models:       fakeModels{list: ModelList{Harness: "claude-code"}},
		Strain:       fakeStrain{},
		Harnesses:    func() []string { return []string{"claude-code", "codex"} },
		Work:         workers.New(l, 4, time.Now),
		WriteEnabled: true,
		Root:         root,
		Broad:        broad,
		BroadDir:     broadDir,
	}
	return d, l
}

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
// never starts in a directory a containment check cannot protect. These cases
// are read jobs, whose behavior does not change with this fix.
func TestWorkerDirDefaultsToRootUnlessBroad(t *testing.T) {
	d := Deps{Root: "/proj"}
	if got, err := d.workerDir("", false); err != nil || got != "/proj" {
		t.Fatalf("empty dir = %q, %v; want the root", got, err)
	}
	if got, err := d.workerDir("  ", false); err != nil || got != "/proj" {
		t.Fatalf("blank dir = %q, %v; want the root", got, err)
	}
	if got, err := d.workerDir("/other", false); err != nil || got != "/other" {
		t.Fatalf("explicit dir = %q, %v; want it kept", got, err)
	}

	broad := Deps{Root: "/", Broad: true}
	if _, err := broad.workerDir("", false); err == nil {
		t.Fatal("a missing dir must be refused when the root is broad")
	}
	// A read job keeps an explicit dir even under a broad root (unchanged).
	if got, err := broad.workerDir("/proj", false); err != nil || got != "/proj" {
		t.Fatalf("read explicit dir under a broad root = %q, %v; want it kept", got, err)
	}

	none := Deps{}
	if _, err := none.workerDir("", false); err == nil {
		t.Fatal("a missing dir must be refused when no root was resolved")
	}
}

// Under write, a broad server root refuses the job even when an explicit dir is
// given: the explicit dir cannot opt out of a broad root. A non-broad root
// keeps the explicit write dir, exactly like a read job.
func TestWorkerDirWriteRefusesBroadServerRoot(t *testing.T) {
	broad := Deps{Root: "/", Broad: true}
	_, err := broad.workerDir("/proj", true)
	if err == nil {
		t.Fatal("a write job must be refused when the server root is broad, even with an explicit dir")
	}
	if !strings.Contains(err.Error(), "broad") || !strings.Contains(err.Error(), "project dir") {
		t.Fatalf("the refusal must name the reason and the fix, got %q", err)
	}

	ok := Deps{Root: "/proj"}
	if got, err := ok.workerDir("/proj/sub", true); err != nil || got != "/proj/sub" {
		t.Fatalf("write explicit dir under a non-broad root = %q, %v; want it kept", got, err)
	}
}

// Under write, an explicit dir that is itself broad is refused even when the
// server root is a safe project directory. The BroadDir port decides; a read
// job with the same dir is kept.
func TestWorkerDirWriteRefusesBroadExplicitDir(t *testing.T) {
	isBroad := func(dir string) bool { return dir == "/" || dir == "/home" || dir == "/home/me" }
	d := Deps{Root: "/proj", BroadDir: isBroad}

	for _, dir := range []string{"/", "/home", "/home/me"} {
		if _, err := d.workerDir(dir, true); err == nil {
			t.Fatalf("a write job with broad explicit dir %q must be refused", dir)
		}
	}
	if _, err := d.workerDir("/", true); err != nil {
		if !strings.Contains(err.Error(), "broad") || !strings.Contains(err.Error(), "project dir") {
			t.Fatalf("the refusal must name the reason and the fix, got %q", err)
		}
	}
	// A safe explicit dir is kept under write.
	if got, err := d.workerDir("/proj/sub", true); err != nil || got != "/proj/sub" {
		t.Fatalf("write explicit safe dir = %q, %v; want it kept", got, err)
	}
	// A read job with the same broad dir is unchanged (kept, not refused).
	if got, err := d.workerDir("/home/me", false); err != nil || got != "/home/me" {
		t.Fatalf("read job with a broad dir = %q, %v; want it kept unchanged", got, err)
	}
}

// Under bypass (write enabled), delegate refuses a write job when the server
// root is broad and when an explicit dir is broad, but still runs a write job
// pointed at a real project dir. Read behavior is covered by the workerDir
// tests above.
func TestDelegateWriteUnderBypassRefusesBroadRunsProject(t *testing.T) {
	isBroad := func(dir string) bool { return dir == "/home/me" }

	// bypass + broad server root: refused before any worker starts.
	d, l := delegateDeps("/", true, isBroad)
	if _, err := d.delegate(context.Background(), json.RawMessage(
		`{"tasks":["do x"],"agent":"claude","mode":"write"}`)); err == nil {
		t.Fatal("bypass write with a broad server root must be refused")
	}
	if len(l.specs) != 0 {
		t.Fatalf("a worker started despite a broad root: %+v", l.specs)
	}

	// bypass + explicit broad dir: refused.
	d, l = delegateDeps("/proj", false, isBroad)
	if _, err := d.delegate(context.Background(), json.RawMessage(
		`{"tasks":["do x"],"agent":"claude","mode":"write","dir":"/home/me"}`)); err == nil {
		t.Fatal("bypass write with a broad explicit dir must be refused")
	}
	if len(l.specs) != 0 {
		t.Fatalf("a worker started despite a broad explicit dir: %+v", l.specs)
	}

	// bypass + project dir: runs, and the worker is a write worker in that dir.
	d, l = delegateDeps("/proj", false, isBroad)
	if _, err := d.delegate(context.Background(), json.RawMessage(
		`{"tasks":["do x"],"agent":"claude","mode":"write","dir":"/proj/sub"}`)); err != nil {
		t.Fatalf("bypass write with a project dir must run: %v", err)
	}
	d.Work.Close()
	if len(l.specs) != 1 || l.specs[0].Dir != "/proj/sub" || !l.specs[0].Write {
		t.Fatalf("expected one write job in /proj/sub, got %+v", l.specs)
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
		"diff":     "Read a worker's diff",
	}
	for _, tl := range ts {
		if want, ok := titles[tl.Name]; !ok || tl.Title != want {
			t.Errorf("%s: Title = %q, want %q", tl.Name, tl.Title, want)
		}
	}
}

// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/accounting"
	"github.com/tiagovilasboas/downshift/internal/execwrap"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// The three sessions of testdata/parity. Hashes are telemetry.HashSessionID of
// the synthetic raw ids "parity-alpha", "parity-beta" and "parity-old".
var paritySessions = []string{"alpha", "beta", "old"}

func parityHash(name string) string { return telemetry.HashSessionID("parity-" + name) }

var parityOffset = regexp.MustCompile(`"@-(\d+)([mhd])"`)

// materializeParityLog copies a golden JSONL file into dst, turning each
// "@-<n><unit>" placeholder into an RFC3339 time that far before now. The
// golden file stays stable while the 30-day window keeps its meaning.
func materializeParityLog(t *testing.T, src, dst string, now time.Time) {
	t.Helper()
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	unit := map[string]time.Duration{"m": time.Minute, "h": time.Hour, "d": 24 * time.Hour}
	out := parityOffset.ReplaceAllFunc(raw, func(m []byte) []byte {
		sub := parityOffset.FindSubmatch(m)
		n, _ := strconv.Atoi(string(sub[1]))
		ts := now.Add(-time.Duration(n) * unit[string(sub[2])]).UTC().Format(time.RFC3339Nano)
		return []byte(strconv.Quote(ts))
	})
	if err := os.WriteFile(dst, out, 0o600); err != nil {
		t.Fatal(err)
	}
}

// parityState points DOWNSHIFT_STATE_DIR at a temp copy of the golden logs and
// returns the logs as every surface will read them.
func parityState(t *testing.T) ([]telemetry.Event, []execwrap.Event) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("DOWNSHIFT_STATE_DIR", dir)
	t.Setenv("DOWNSHIFT_EVENT_LOG", "")
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	now := time.Now()
	materializeParityLog(t, filepath.Join("testdata", "parity", "events.jsonl"), filepath.Join(dir, "events.jsonl"), now)
	materializeParityLog(t, filepath.Join("testdata", "parity", "exec-events.jsonl"), filepath.Join(dir, execwrap.EventsFileName), now)
	events, err := telemetry.ReadEvents()
	if err != nil || len(events) == 0 {
		t.Fatalf("golden event log: %d events, err %v", len(events), err)
	}
	execEvents, err := accounting.ReadExecEvents()
	if err != nil || len(execEvents) == 0 {
		t.Fatalf("golden exec log: %d events, err %v", len(execEvents), err)
	}
	return events, execEvents
}

// flattenJSON turns any JSON value into dotted-path leaves.
func flattenJSON(prefix string, v any, out map[string]string) {
	switch x := v.(type) {
	case map[string]any:
		for k, c := range x {
			flattenJSON(prefix+"."+k, c, out)
		}
	case []any:
		for i, c := range x {
			flattenJSON(fmt.Sprintf("%s[%d]", prefix, i), c, out)
		}
	default:
		out[prefix] = fmt.Sprint(x)
	}
}

func flatOf(t *testing.T, v any) map[string]string {
	t.Helper()
	raw, ok := v.([]byte)
	if !ok {
		var err error
		if raw, err = json.Marshal(v); err != nil {
			t.Fatal(err)
		}
	}
	var tree any
	if err := json.Unmarshal(raw, &tree); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, raw)
	}
	out := map[string]string{}
	flattenJSON("", tree, out)
	return out
}

// requireSame fails once per differing field, naming the surface and the value
// each side gave, so a drift reads as "report.compactor.tokens: want 75, got 74".
func requireSame(t *testing.T, surface string, got, want any) {
	t.Helper()
	g, w := flatOf(t, got), flatOf(t, want)
	keys := map[string]bool{}
	for k := range g {
		keys[k] = true
	}
	for k := range w {
		keys[k] = true
	}
	var sorted []string
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)
	for _, k := range sorted {
		gv, gok := g[k]
		wv, wok := w[k]
		if gok != wok || gv != wv {
			t.Errorf("%s drifts from accounting.Compute at %s: surface=%q (present=%v), Compute=%q (present=%v)", surface, k, gv, gok, wv, wok)
		}
	}
}

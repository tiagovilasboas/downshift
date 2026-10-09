// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package codex_test

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/adapters/codex"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/quota"
)

func TestHandleQuotaGovernsRewriteAndPreservesSiblings(t *testing.T) {
	t.Setenv("DOWNSHIFT_QUOTA_FILE", filepath.Join(t.TempDir(), "missing"))
	now := time.Now()
	used := 10.0
	s := quota.Snapshot{Version: 1, Harness: "codex", Source: "codex-transcript", ObservedAt: now.Add(-time.Second), ExpiresAt: now.Add(time.Minute), Windows: []quota.Window{{ID: "shared", Scope: "harness", UsedPercent: &used, ResetsAt: now.Add(time.Hour)}}}
	ev := withCatalogSession(codex.Event{Model: catID(core.TierFrontier), ToolInput: json.RawMessage(`{"message":"rename the userId variable to userIdentifier","task_name":"worker_agent_rename","fork_turns":"none","custom":{"keep":true}}`), UsageQuota: &s})
	out, _, d := codex.Handle(ev, cat)
	m := decodeUpdated(t, out)
	if m == nil || m["model"] != catID(core.TierSmall) || m["custom"] == nil || m["fork_turns"] != "none" || d.QuotaStatus != "available" {
		t.Fatalf("funded rewrite lost siblings %+v %+v", m, d)
	}
	for _, tc := range []struct {
		name   string
		change func()
	}{
		{"exhausted", func() { used = 100 }},
		{"stale", func() { used = 10; s.ExpiresAt = now.Add(-time.Millisecond) }},
		{"cross harness", func() { s.ExpiresAt = now.Add(time.Minute); s.Harness = "cursor" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.change()
			out, _, _ := codex.Handle(ev, cat)
			if decodeUpdated(t, out) != nil {
				t.Fatal("unfunded rewrite/effort emitted")
			}
		})
	}
	ev.UsageQuota = nil
	ev.TranscriptPath = filepath.Join(t.TempDir(), "missing.jsonl")
	out, _, d = codex.Handle(ev, cat)
	if decodeUpdated(t, out) != nil || d.QuotaStatus != "unknown" {
		t.Fatal("missing explicit transcript reopened legacy quota")
	}
}

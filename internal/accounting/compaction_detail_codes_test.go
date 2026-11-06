// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"reflect"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/execwrap"
)

// The passthrough details and metachar classes are counted per harness, most
// frequent first; rows without the optional fields add nothing.
func TestBuildCompactionDetail_DetailCodes(t *testing.T) {
	now := time.Now()
	ts := now.Format(time.RFC3339)
	un := execwrap.PassthroughUnrecognized
	evs := []execwrap.Event{
		{TS: ts, Harness: "claude-code", Family: "go-test", Passthrough: un, PassthroughDetail: execwrap.DetailCR},
		{TS: ts, Harness: "claude-code", Family: "go-test", Passthrough: un, PassthroughDetail: execwrap.DetailCR},
		{TS: ts, Harness: "claude-code", Family: "go-test", Passthrough: un, PassthroughDetail: execwrap.DetailANSI},
		{TS: ts, Harness: "claude-code", Family: "go-test", Passthrough: un},
	}
	skips := []execwrap.SkipEvent{
		{TS: ts, Harness: "claude-code", Reason: execwrap.SkipMetachar, Metachar: execwrap.MetaPipe},
		{TS: ts, Harness: "claude-code", Reason: execwrap.SkipMetachar, Metachar: execwrap.MetaPipe},
		{TS: ts, Harness: "claude-code", Reason: execwrap.SkipMetachar, Metachar: execwrap.MetaAnd},
		{TS: ts, Harness: "claude-code", Reason: execwrap.SkipMetachar},
	}
	got := BuildCompactionDetail(evs, skips, Scope{Kind: ScopeAll}, now)
	h := got.Harnesses[0]
	wantDetails := []ReasonCount{{Reason: execwrap.DetailCR, Count: 2}, {Reason: execwrap.DetailANSI, Count: 1}}
	wantMeta := []ReasonCount{{Reason: execwrap.MetaPipe, Count: 2}, {Reason: execwrap.MetaAnd, Count: 1}}
	if !reflect.DeepEqual(h.PassthroughDetails, wantDetails) {
		t.Errorf("details = %+v, want %+v", h.PassthroughDetails, wantDetails)
	}
	if !reflect.DeepEqual(h.SkipMetachars, wantMeta) {
		t.Errorf("metachars = %+v, want %+v", h.SkipMetachars, wantMeta)
	}
	if h.NotCompacted[0].Count != 4 || h.Skipped[0].Count != 4 {
		t.Errorf("reason totals changed: %+v %+v", h.NotCompacted, h.Skipped)
	}
}

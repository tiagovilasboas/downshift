// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package fromcore

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func TestReasonCodes_AgentTypeHint(t *testing.T) {
	d := core.RouteAgent("look around the repo and tell me how the router is wired", "claude-code", "Explore", "claude-opus-5-5", catalog.Load())
	found := false
	for _, c := range ReasonCodes(d) {
		found = found || c == telemetry.ReasonAgentTypeHint
	}
	if !found {
		t.Fatalf("codes %v lack %s", ReasonCodes(d), telemetry.ReasonAgentTypeHint)
	}
}

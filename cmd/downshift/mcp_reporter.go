// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"time"

	"github.com/tiagovilasboas/downshift/internal/discovery"
	"github.com/tiagovilasboas/downshift/internal/mcp/tools"
)

// mcpReporter hands the model ids the orchestrator reports to the discovery
// cache. The cache decides what is kept: a fresh detection always wins.
type mcpReporter struct {
	cat discovery.Catalog
	now func() time.Time
}

func (r mcpReporter) Report(ctx context.Context, harness string, ids []string) (tools.ReportOutcome, error) {
	if err := ctx.Err(); err != nil {
		return tools.ReportOutcome{}, err
	}
	path, err := discovery.Path()
	if err != nil {
		return tools.ReportOutcome{}, err
	}
	res, err := discovery.MergeReported(path, harness, ids, r.now(), r.cat)
	if err != nil {
		return tools.ReportOutcome{}, err
	}
	return tools.ReportOutcome{Accepted: res.Accepted, Written: res.Written, Reason: res.Reason}, nil
}

// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"fmt"

	"github.com/tiagovilasboas/downshift/internal/routing/policy"
	"github.com/tiagovilasboas/downshift/internal/workers"
)

// RefusalQuota: quota is at or past the stop line, so no worker was started.
const RefusalQuota = "quota"

// quotaStopLine is the configured stop line, or the default when none is wired.
func (d Deps) quotaStopLine() float64 {
	if d.QuotaStop != nil {
		if n := d.QuotaStop(); n > 0 {
			return n
		}
	}
	return policy.DefaultQuotaStopPct
}

// quotaStop refuses a call while the agent's provider is rate limited or past
// the stop line. An unreadable quota never refuses: doubt stays allow.
func (d Deps) quotaStop(ctx context.Context, agent string, rec *delegateRecord) error {
	if d.Strain == nil {
		return nil
	}
	entries, err := d.Strain.Strain(ctx, []string{workers.HarnessFor(agent)})
	if err != nil {
		return nil
	}
	readings := make([]policy.QuotaReading, 0, len(entries))
	for _, e := range entries {
		readings = append(readings, policy.QuotaReading{Provider: e.Provider, UsedPct: e.UsedPct, RateLimited: e.RateLimited, Known: e.Headroom != "unknown"})
	}
	stop, ok := policy.CheckQuotaStop(d.quotaStopLine(), readings)
	if !ok {
		return nil
	}
	rec.code = RefusalQuota
	return fmt.Errorf("no worker started (%s): %s; wait for the window to reset, work without workers, or raise orchestration.quota_stop_pct or %s", RefusalQuota, stop.Reason(), policy.QuotaStopEnv)
}

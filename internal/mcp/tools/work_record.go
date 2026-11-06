// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// RefusalWorkersOff and RefusalWriteOff are the refusal codes for a delegate
// call the server's worker mode does not allow.
const (
	RefusalWorkersOff = "workers_off"
	RefusalWriteOff   = "write_off"
)

// workersOnFix is the fix line a refused delegate carries when workers are off.
const workersOnFix = "fix: the owner turns workers on with `downshift setup --workers read`, orchestration.workers in ~/.downshift/policy.json, or DOWNSHIFT_WORKERS=read; until then use your own subagents"

// delegateRecord collects what one delegate call records for adoption: counts
// and a refusal code, never the task text or the directory.
type delegateRecord struct {
	code   string
	counts CallCounts
}

// accepted counts the jobs a call started.
func (r *delegateRecord) accepted(jobs int, write bool, env []string) {
	r.counts.Tasks = jobs
	if write {
		r.counts.Write = jobs
	}
	if len(env) > 0 {
		r.counts.GatewayRouted = jobs
	}
}

// delegate runs one delegate call and records it, accepted or refused.
func (d Deps) delegate(ctx context.Context, raw json.RawMessage) (Result, error) {
	var rec delegateRecord
	res, err := d.delegateRun(ctx, raw, &rec)
	if err != nil && rec.code == "" {
		rec.code = "invalid"
	}
	d.recordCall("delegate", rec.code, rec.counts)
	return res, err
}

// agentEnabled refuses an agent the server's resolved worker mode does not
// run. A Deps with no resolved mode (Mode "") predates the resolver and keeps
// the launcher's own refusal.
func (d Deps) agentEnabled(agent string, rec *delegateRecord) error {
	if d.Workers.Mode == "" || d.Workers.Plan().Enabled(agent) {
		return nil
	}
	rec.code = RefusalWorkersOff
	if d.Workers.Mode == "off" {
		return fmt.Errorf("worker tools are off on this server (%s); %s", RefusalWorkersOff, workersOnFix)
	}
	return fmt.Errorf("agent %s is not enabled on this server (%s): its read-only mode has no recorded evidence yet; %s", agent, RefusalWorkersOff, workersOnFix)
}

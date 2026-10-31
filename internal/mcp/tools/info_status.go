// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"sort"
	"time"

	"github.com/tiagovilasboas/downshift/internal/workers"
)

const statusSchema = `{"type":"object","properties":{},"additionalProperties":false}`

// maxWorkersShown caps the finished workers the status view lists. Active
// workers (queued or running) are always shown; finished ones are shown newest
// first up to this cap, and the rest are counted as omitted, so a long session
// never grows the result without bound (spec R21).
const maxWorkersShown = 10

type statusData struct {
	Server               string        `json:"server"`
	Version              string        `json:"version"`
	Playbooks            int           `json:"playbooks"`
	PlaybooksUnavailable bool          `json:"playbooks_unavailable,omitempty"`
	Workers              *workersBlock `json:"workers,omitempty"`
}

// workersBlock is the status view of this session's delegated workers. It is
// present whenever the server has a worker queue, even with nothing running
// (an empty block means "nothing is delegated", not an error). It is absent
// when the server runs no queue at all.
type workersBlock struct {
	// Active counts the workers still queued or running.
	Active int `json:"active"`
	// Shown lists the active workers plus the most recent finished ones.
	Shown []workerLine `json:"shown"`
	// Omitted counts the finished workers past the cap, newest kept.
	Omitted int `json:"omitted,omitempty"`
}

// workerLine is one worker in the status view. It carries no prompt and no
// model name: the job records the agent, not the model it ran, so the view
// names the harness from the agent and never a model (owner decision: no model
// cravado unless the job records what it ran).
type workerLine struct {
	ID       string          `json:"id"`
	State    string          `json:"state"`
	Agent    string          `json:"agent"`
	Harness  string          `json:"harness"`
	Mode     string          `json:"mode"`
	ElapsedS int             `json:"elapsed_seconds"`
	Tokens   *workers.Tokens `json:"tokens,omitempty"`
	Summary  string          `json:"summary,omitempty"`
}

func (d Deps) statusTool() Tool {
	return d.build("status",
		"Reports the Downshift server version, how many playbooks are active, the quota strain of every harness, and the workers this session has delegated (id, state, agent and harness, read or write, elapsed time, tokens once known, and a short result or error summary). It lists the active workers plus the most recent finished ones and says how many it left out. Call it to check that Downshift is reachable, how much headroom each provider has, and what is running, without calling collect and waiting.",
		statusSchema, nil,
		func(context.Context, map[string]string) (outcome, error) {
			s := statusData{Server: serverName, Version: d.Version}
			if sets, err := d.sets(); err != nil {
				s.PlaybooksUnavailable = true
			} else {
				s.Playbooks = len(sets)
			}
			s.Workers = d.workersBlock()
			return outcome{data: s, harnesses: d.harnessIDs()}, nil
		})
}

// workersBlock reads a snapshot of the session's workers and shapes it for the
// status view: active workers first, then finished ones newest first up to the
// cap, with the rest counted as omitted. It returns nil when the server has no
// queue, so the status omits the block entirely. The read is a single
// non-blocking call into the queue (Workers holds its lock only briefly), so
// status never serializes behind a waiting collect or cancel (spec R22).
func (d Deps) workersBlock() *workersBlock {
	if d.Work == nil {
		return nil
	}
	views := d.Work.Workers()
	b := &workersBlock{Shown: []workerLine{}}

	var active, finished []workers.WorkerView
	for _, v := range views {
		if v.State.Finished() {
			finished = append(finished, v)
		} else {
			active = append(active, v)
		}
	}
	b.Active = len(active)

	// Finished workers newest first. The queue returns start order, so a higher
	// id is more recent; a stable sort keeps that order for ties (there are none).
	sort.SliceStable(finished, func(i, j int) bool { return finished[i].ID > finished[j].ID })
	if len(finished) > maxWorkersShown {
		b.Omitted = len(finished) - maxWorkersShown
		finished = finished[:maxWorkersShown]
	}

	for _, v := range active {
		b.Shown = append(b.Shown, lineOf(v))
	}
	for _, v := range finished {
		b.Shown = append(b.Shown, lineOf(v))
	}
	return b
}

// lineOf shapes one worker view into a status line.
func lineOf(v workers.WorkerView) workerLine {
	mode := "read"
	if v.Write {
		mode = "write"
	}
	return workerLine{
		ID:       v.ID,
		State:    string(v.State),
		Agent:    v.Agent,
		Harness:  v.Harness,
		Mode:     mode,
		ElapsedS: int(v.Elapsed / time.Second),
		Tokens:   v.Tokens,
		Summary:  v.Summary,
	}
}

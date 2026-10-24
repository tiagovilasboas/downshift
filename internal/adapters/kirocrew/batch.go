// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package kirocrew

import (
	"fmt"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// spawnRunTool is the KiroCrew spawn tool whose tasks[] array is a batch.
const spawnRunTool = "spawn_run"

// batchEntry is one routable member of a spawn_run tasks[] array. Index is its
// 1-based position in the array, so a block names the entry the agent sent.
// Model is empty when the entry does not name one.
type batchEntry struct {
	index int
	task  string
	model string
}

// hasTasksBatch reports whether a spawn_run input carries a non-null tasks
// field. KiroCrew prefers tasks over the top-level task, so a present tasks
// field selects the batch path and the top-level fields are ignored.
func hasTasksBatch(ti map[string]any) bool {
	v, ok := ti["tasks"]
	return ok && v != nil
}

// parseBatch reads a tasks[] array. Each entry is a bare prompt string or an
// object {task, model, reasoning_effort}; blank tasks are skipped, as KiroCrew
// skips them. ok is false when the array is malformed (not an array, an entry
// that is neither string nor object, or a task or model that is not a string),
// so the whole call fails open.
func parseBatch(raw any) (entries []batchEntry, ok bool) {
	items, isArray := raw.([]any)
	if !isArray {
		return nil, false
	}
	for i, item := range items {
		entry := batchEntry{index: i + 1}
		switch v := item.(type) {
		case string:
			entry.task = v
		case map[string]any:
			task, isString := v["task"].(string)
			if !isString {
				return nil, false
			}
			entry.task = task
			if m, present := v["model"]; present && m != nil {
				model, isModelString := m.(string)
				if !isModelString {
					return nil, false
				}
				entry.model = model
			}
		default:
			return nil, false
		}
		if strings.TrimSpace(entry.task) != "" {
			entries = append(entries, entry)
		}
	}
	return entries, true
}

// handleBatch routes every entry of a spawn_run batch on its own. KiroCrew's
// hook is allow/block for the whole call and cannot rewrite one entry, so any
// entry that should respawn blocks the call; the message names each such entry.
// Every routed entry yields one decision, carried in the Output for telemetry.
func handleBatch(ev Event, ti map[string]any, res core.Resolver) (Output, string, core.Decision) {
	entries, ok := parseBatch(ti["tasks"])
	if !ok || len(entries) == 0 {
		return allow(), "", core.Decision{}
	}
	decisions := make([]core.Decision, 0, len(entries))
	var lines []string
	smallTarget := false
	for _, e := range entries {
		rt := routeSpawn(ev, res, e.task, e.model)
		decisions = append(decisions, rt.decision)
		if !rt.block {
			continue
		}
		lines = append(lines, fmt.Sprintf("entry %d: respawn with model=%s (tier %s)",
			e.index, rt.target.ID, rt.target.Tier))
		smallTarget = smallTarget || rt.target.Tier == core.TierSmall
	}
	out := Output{decisions: decisions}
	if len(lines) == 0 {
		return out, "", decisions[0]
	}
	note := fmt.Sprintf("spawn_run batch: %d of %d entries should respawn", len(lines), len(entries))
	msg := "downshift: " + note + "\n" + strings.Join(lines, "\n") +
		"\nRespawn the batch with those entries changed to the models named; the other entries are fine."
	if smallTarget {
		msg += smallTierHint
	}
	out.Block = true
	out.Message = msg
	return out, note, decisions[0]
}

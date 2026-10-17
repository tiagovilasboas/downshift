// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

// Entry values recorded on decision events. They name the spawn-time path that
// called the shared router; they are not harness ids and carry no task text.
const (
	EntryHook = "hook" // settings or plugin PreToolUse hook
	EntryMod  = "mod"  // in-process mod (for example Claude Code agent.spawn)
)

// NormalizeEntry returns a known entry label. Empty or unknown values become
// EntryHook so new decision rows are always attributable.
func NormalizeEntry(entry string) string {
	switch entry {
	case EntryMod:
		return EntryMod
	default:
		return EntryHook
	}
}

// DuplicateToolUseCount returns how many distinct tool_use_hash values appear
// on more than one decision row in events. Two decisions for one hash mean both
// entry points routed the same spawn (double routing). Hashes are compared
// within a harness; empty hashes are ignored. Prompt-free: only closed fields.
func DuplicateToolUseCount(events []Event) int {
	type key struct{ harness, hash string }
	counts := map[key]int{}
	for _, ev := range events {
		if !IsDecisionRow(ev) || ev.ToolUseHash == "" {
			continue
		}
		counts[key{ev.Harness, ev.ToolUseHash}]++
	}
	n := 0
	for _, c := range counts {
		if c > 1 {
			n++
		}
	}
	return n
}

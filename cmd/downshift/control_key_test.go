// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

// The control group must assign one spawn to one side. Claude Code, Cursor and
// Codex send a tool-use id but no correlation id, so before the fix every hook
// call drew a fresh random routing id: the settings hook and the mod entry for
// the same spawn could land on opposite sides, and a retried call could flip.

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func isControl(ev telemetry.Event) bool {
	for _, c := range ev.ReasonCodes {
		if c == telemetry.ReasonControl {
			return true
		}
	}
	return false
}

func TestControlGroup_SameToolUseLandsOnSameSide(t *testing.T) {
	const prompt = "rename the userId variable to userIdentifier"
	for _, tc := range []struct {
		harness string
		// payloads returns the copies one spawn can produce, all with the same tool-use id.
		payloads func(toolUseID, model string) []string
	}{
		{"claude-code", func(id, model string) []string {
			hook := `{"hook_event_name":"PreToolUse","tool_name":"Agent","session_id":"s-key","tool_use_id":"` + id +
				`","tool_input":{"prompt":"` + prompt + `","subagent_type":"general-purpose","model":"` + model + `"}}`
			mod := `{"hook_event_name":"PreToolUse","tool_name":"Agent","session_id":"s-key","tool_use_id":"` + id +
				`","entry":"mod","parent_model":"` + model + `","tool_input":{"prompt":"` + prompt + `","description":"rename","subagent_type":"general-purpose"}}`
			return []string{hook, mod}
		}},
		{"cursor", func(id, model string) []string {
			p := `{"hook_event_name":"preToolUse","tool_name":"Task","tool_use_id":"` + id + `","model_id":"` + model +
				`","tool_input":{"task":"` + prompt + `","model":"` + model + `"}}`
			return []string{p, p}
		}},
		{"codex", func(id, model string) []string {
			p := `{"hook_event_name":"PreToolUse","tool_name":"spawn_agent","session_id":"s-key","tool_use_id":"` + id +
				`","model":"` + model + `","tool_input":{"message":"` + prompt + `","model":"` + model + `"}}`
			return []string{p, p}
		}},
	} {
		t.Run(tc.harness, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("DOWNSHIFT_QUIET", "1")
			eventLog := filepath.Join(t.TempDir(), "events.jsonl")
			t.Setenv("DOWNSHIFT_EVENT_LOG", eventLog)
			t.Setenv("DOWNSHIFT_CONTROL_SHARE", "0.5")
			small := cmdCat.ModelFor(tc.harness, core.TierSmall).ID
			mid := cmdCat.ModelFor(tc.harness, core.TierMid).ID
			frontier := cmdCat.ModelFor(tc.harness, core.TierFrontier).ID
			setSessionAllowlist(t, tc.harness, []string{small, mid, frontier})

			// At share 0.5 a random id splits a pair with probability 1/2, so 24
			// spawns pass by chance with probability 2^-24.
			const spawns = 24
			for i := 0; i < spawns; i++ {
				for _, p := range tc.payloads(fmt.Sprintf("toolu_key_%02d", i), frontier) {
					captureStdout(func() {
						captureStderr(func() {
							if rc := harnessRegistry[tc.harness].Hook(strings.NewReader(p), cmdCat); rc != 0 {
								t.Errorf("hook rc = %d", rc)
							}
						})
					})
				}
			}
			events, err := telemetry.ReadEventsFrom(eventLog)
			if err != nil {
				t.Fatal(err)
			}
			side := map[string][]bool{}
			for _, ev := range events {
				if telemetry.IsDecisionRow(ev) && ev.ToolUseHash != "" {
					side[ev.ToolUseHash] = append(side[ev.ToolUseHash], isControl(ev))
				}
			}
			if len(side) != spawns {
				t.Fatalf("decisions for %d tool-use ids, want %d (events %d)", len(side), spawns, len(events))
			}
			control := 0
			for hash, s := range side {
				if len(s) != 2 || s[0] != s[1] {
					t.Errorf("tool_use_hash %s…: control flags %v, want two equal", hash[:8], s)
				}
				if s[0] {
					control++
				}
			}
			if control == 0 || control == spawns {
				t.Errorf("control spawns = %d of %d at share 0.5: assignment is not using the id", control, spawns)
			}
		})
	}
}

// The precedence of the control key: a valid harness correlation id, then the
// tool-use id, then the per-call correlation id. Only the key is hashed, never logged.
func TestControlKey_Precedence(t *testing.T) {
	const corr = "0123456789abcdef0123456789abcdef"
	for _, tc := range []struct {
		name      string
		sent      bool
		toolUseID string
		wantTool  bool
	}{
		{"harness correlation id wins", true, "toolu_1", false},
		{"tool-use id when no correlation id", false, "toolu_1", true},
		{"per-call id when neither", false, "", false},
		{"invalid tool-use id is ignored", false, "bad id with spaces", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := controlKey(corr, tc.sent, tc.toolUseID)
			if tc.wantTool {
				if got == corr || got == "" || strings.Contains(got, tc.toolUseID) {
					t.Errorf("controlKey = %q, want a hash of the tool-use id", got)
				}
				if again := controlKey("fedcba9876543210fedcba9876543210", false, tc.toolUseID); again != got {
					t.Errorf("tool-use key depends on the per-call id: %q vs %q", got, again)
				}
				return
			}
			if got != corr {
				t.Errorf("controlKey = %q, want the correlation id %q", got, corr)
			}
		})
	}
}

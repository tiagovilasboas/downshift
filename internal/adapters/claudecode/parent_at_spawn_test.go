// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode

import (
	"os"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/testenv"
)

const (
	sonnetTurn = `{"type":"assistant","message":{"model":"m-sonnet","content":[{"type":"text"}]}}` + "\n"
	opusCall   = `{"type":"assistant","message":{"model":"m-opus","content":[{"type":"tool_use","id":"toolu_AAA"}]}}` + "\n"
)

// The baseline is the parent's model when it issued this very spawn call. After a
// model switch the last assistant model in the transcript is the previous turn's.
func TestParentModelAtSpawn(t *testing.T) {
	withTranscriptWait(t, 120*time.Millisecond, 20*time.Millisecond)
	cases := []struct {
		name, body, call, want string
	}{
		{"the call's own message wins over the previous turn", sonnetTurn + opusCall, "toolu_AAA", "m-opus"},
		{"the call's message wins over a later sibling turn", opusCall + sonnetTurn, "toolu_AAA", "m-opus"},
		{"a result line carrying the id is not the call", sonnetTurn + `{"type":"user","message":{"model":"m-user","content":[{"type":"tool_result","tool_use_id":"toolu_AAA"}]}}` + "\n", "toolu_AAA", "m-sonnet"},
		{"a call that never lands falls back to the last model", sonnetTurn, "toolu_AAA", "m-sonnet"},
		{"no call id reads the last model", sonnetTurn + opusCall, "", "m-opus"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parentModelAtSpawn(writeTranscript(t, tc.body), tc.call); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// The transcript is written asynchronously: the message that issued the call can
// land after the hook starts, and the hook waits for it, bounded.
func TestParentModelAtSpawn_WaitsForTheCallToLand(t *testing.T) {
	withTranscriptWait(t, 600*time.Millisecond, 20*time.Millisecond)
	path := writeTranscript(t, sonnetTurn)
	go func() {
		time.Sleep(100 * time.Millisecond)
		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
		if err == nil {
			_, _ = f.WriteString(opusCall)
			_ = f.Close()
		}
	}()
	if got := parentModelAtSpawn(path, "toolu_AAA"); got != "m-opus" {
		t.Errorf("got %q, want the model of the call that landed", got)
	}
}

// With a call id and no transcript file there is nothing to wait for.
func TestParentModelAtSpawn_NoFileNoWait(t *testing.T) {
	withTranscriptWait(t, 400*time.Millisecond, 20*time.Millisecond)
	start := time.Now()
	if got := parentModelAtSpawn(t.TempDir()+"/absent.jsonl", "toolu_AAA"); got != "" {
		t.Fatalf("got %q", got)
	}
	testenv.CheckBudget(t, "wait for a file that does not exist", time.Since(start), 200*time.Millisecond)
}

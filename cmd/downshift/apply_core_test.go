// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/hookctx"
)

const (
	profilePolicy = `{"version":1,"routing":{"profiles":[{"name":"reviewer","tier":"small","harness":"claude-code"}]}}`
	hintDownText  = "[tier=small] "
)

// TestApplierMatchesTheHook is the shared-layer proof: for the same spawn the
// hook decider and the Applier (used by `route` and the MCP tool) land on the
// same model, and the Applier's `applied` says whether that model moved.
func TestApplierMatchesTheHook(t *testing.T) {
	cases := []struct {
		name      string
		policy    string
		agentType string
		tier      string
		model     string
		wantMoved bool
		wantID    string // selected model when moved
	}{
		{"nothing declared keeps", "", "", "", frontierID, false, ""},
		{"a declared profile moves", profilePolicy, "reviewer", "", frontierID, true, smallID},
		{"a declared rule moves", gatedRule, "", "", frontierID, true, smallID},
		{"a tier hint goes down", "", "", "small", frontierID, true, smallID},
		{"a tier hint never goes up", "", "", "frontier", smallID, false, ""},
		{"unknown model keeps", gatedRule, "", "small", "no-such-model", false, ""},
		{"invalid policy keeps", `{"version":1,"routing":{"rules":[{"id":"x"}]}}`, "", "small", frontierID, false, ""},
		{"malformed policy keeps", `{not json`, "", "small", frontierID, false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("DOWNSHIFT_STATE_DIR", t.TempDir())
			t.Setenv("DOWNSHIFT_QUOTA_FILE", filepath.Join(t.TempDir(), "quota.json"))
			if tc.policy != "" {
				writeFile(t, filepath.Join(dir, ".downshift", "policy.json"), tc.policy)
			}
			t.Cleanup(hookctx.SetProjectDir(dir))

			req := core.Request{Harness: "claude-code", CurrentModelID: tc.model, AgentType: tc.agentType, Resolver: cmdCat, Prompt: "plain task"}
			if tc.tier != "" {
				req.Prompt = hintDownText + req.Prompt
			}
			hook := routingDecider{now: func() time.Time { return deciderNow }}.Decide(req)
			hookID := tc.model
			if hook.Verdict == core.VerdictDownshift || hook.Verdict == core.VerdictUpshift {
				hookID = hook.Model.ID
			}

			res, err := spawnApplier{cat: cmdCat, now: func() time.Time { return deciderNow }}.Apply(context.Background(), ApplyInput{
				Harness: "claude-code", RequestedModel: tc.model, AgentType: tc.agentType, TierHint: tc.tier, ProjectDir: dir,
			})
			if err != nil {
				t.Fatal(err)
			}
			if res.SelectedModel != hookID {
				t.Errorf("applier selected %q, hook decided %q", res.SelectedModel, hookID)
			}
			if res.Applied != tc.wantMoved {
				t.Errorf("applied = %v, want %v (reason %q)", res.Applied, tc.wantMoved, res.Reason)
			}
			if tc.wantMoved && res.SelectedModel != tc.wantID {
				t.Errorf("selected %q, want %q", res.SelectedModel, tc.wantID)
			}
			if res.Reason == "" || res.Verdict == "" || res.Note == nil {
				t.Errorf("result lacks reason, verdict or note: %+v", res)
			}
		})
	}
}

func TestApplierRespectsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (spawnApplier{cat: cmdCat, now: time.Now}).Apply(ctx, ApplyInput{Harness: "claude-code"}); err == nil {
		t.Fatal("a cancelled context must return an error")
	}
}

func TestApplierNoteHoldsNoText(t *testing.T) {
	t.Setenv("DOWNSHIFT_STATE_DIR", t.TempDir())
	res, _ := spawnApplier{cat: cmdCat, now: time.Now}.Apply(context.Background(), ApplyInput{
		Harness: "claude-code", RequestedModel: frontierID, TierHint: "small", ProjectDir: t.TempDir(),
	})
	for k, v := range res.Note {
		switch v.(type) {
		case string, float64, bool:
		default:
			t.Errorf("note[%s] has type %T, want enum, number or bool", k, v)
		}
	}
	if strings.Contains(res.Reason, "[tier=") {
		t.Errorf("reason leaks the hint prefix: %q", res.Reason)
	}
}

func TestApplyRefusesAnUnknownTierOrEffortWithoutEchoingIt(t *testing.T) {
	a := spawnApplier{cat: cmdCat, now: func() time.Time { return deciderNow }}
	for _, in := range []ApplyInput{
		{Harness: "claude-code", TierHint: "huge"},
		{Harness: "claude-code", Effort: "zz-secret-looking"},
	} {
		_, err := a.Apply(context.Background(), in)
		var bad *ApplyInputError
		if !errors.As(err, &bad) {
			t.Fatalf("Apply(%+v) error = %v, want an ApplyInputError", in, err)
		}
		if strings.Contains(bad.Msg, "huge") || strings.Contains(bad.Msg, "zz-secret") {
			t.Fatalf("the refusal echoed its input: %q", bad.Msg)
		}
	}
	for _, e := range effortNames {
		if _, err := a.Apply(context.Background(), ApplyInput{Harness: "claude-code", Effort: e}); err != nil {
			t.Errorf("effort %q should be accepted: %v", e, err)
		}
	}
}

func TestApplyNamesAnInvalidProjectPolicyInTheReason(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".downshift", "policy.json"), `{not json`)
	t.Setenv("DOWNSHIFT_STATE_DIR", t.TempDir())
	a := spawnApplier{cat: cmdCat, now: func() time.Time { return deciderNow }}
	r, err := a.Apply(context.Background(), ApplyInput{Harness: "claude-code", RequestedModel: frontierID, ProjectDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if r.Applied || !strings.Contains(r.Reason, "policy") {
		t.Fatalf("an invalid policy must keep the model and say so: %+v", r)
	}
}

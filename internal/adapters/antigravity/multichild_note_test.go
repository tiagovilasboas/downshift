package antigravity_test

import (
	"encoding/json"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/antigravity"
	"github.com/tiagovilasboas/downshift/internal/core"
)

const mcTrivial = "fix typo in README"
const mcRiskTrivial = "fix typo in the auth token secret handler"

func mcEv(args string, ids ...string) antigravity.Event {
	ev := antigravity.Event{ToolCall: antigravity.ToolCall{Name: "invoke_subagent", Args: json.RawMessage(args)}}
	if len(ids) > 0 {
		ev.SessionModels = &ids
	} else {
		ev = withWireSession(ev)
	}
	return ev
}

// One child rewritten, the last child held by a guardrail: the returned
// decision (what the runner records as rewrite_emitted) must describe a child
// that was actually rewritten, not the held one.
func TestMultiChild_HeldLastChildIsNotReportedAsTheRewrite(t *testing.T) {
	args := `{"Subagents":[{"Prompt":"` + mcTrivial + `","Model":"pro"},{"Prompt":"` + mcRiskTrivial + `","Model":"pro"}]}`
	out, note, d := antigravity.Handle(mcEv(args), res)
	subs := decodeOverwriteSubagents(t, out)
	if len(subs) != 2 || subs[0]["Model"] != "flash_lite" || subs[1]["Model"] != "pro" {
		t.Fatalf("setup: want child0 rewritten, child1 held; got %v", subs)
	}
	if len(d.Corrections) > 0 && note != "" {
		t.Fatalf("returned decision is the HELD child (corrections=%v, verdict=%v, model=%s) yet note=%q marks the event as a rewrite", d.Corrections, d.Verdict, d.Model.ID, note)
	}
}

// One child rewritten, the last child already on the right gear (OK).
func TestMultiChild_OKLastChildIsNotReportedAsTheRewrite(t *testing.T) {
	heavy := "rearchitect the distributed consensus protocol and multi-region database replication"
	args := `{"Subagents":[{"Prompt":"` + mcTrivial + `","Model":"pro"},{"Prompt":"` + heavy + `","Model":"pro"}]}`
	out, note, d := antigravity.Handle(mcEv(args), res)
	subs := decodeOverwriteSubagents(t, out)
	if len(subs) != 2 || subs[0]["Model"] != "flash_lite" {
		t.Fatalf("setup: want child0 rewritten; got %v", subs)
	}
	if d.Verdict == core.VerdictOK && note != "" {
		t.Fatalf("returned decision verdict=OK model=%s (unchanged child) but note=%q marks a rewrite", d.Model.ID, note)
	}
}

// The decision returned for a rewritten child must name the model written.
func TestMultiChild_DecisionModelMatchesWrittenModel(t *testing.T) {
	args := `{"Subagents":[{"Prompt":"` + mcTrivial + `","Model":"pro"}]}`
	out, _, d := antigravity.Handle(mcEv(args, "flash", "pro"), res)
	subs := decodeOverwriteSubagents(t, out)
	if len(subs) != 1 {
		t.Fatalf("no overwrite")
	}
	if subs[0]["Model"] != d.Model.ID {
		t.Fatalf("wrote Model=%v but decision.Model.ID=%q (telemetry to_model)", subs[0]["Model"], d.Model.ID)
	}
}

func TestMultiChild_HeldFirstChildDoesNotLeak(t *testing.T) {
	args := `{"Subagents":[{"Prompt":"` + mcRiskTrivial + `","Model":"pro"},{"Prompt":"` + mcTrivial + `","Model":"pro"},{"Prompt":"` + mcTrivial + `","Model":"pro"}]}`
	out, _, _ := antigravity.Handle(mcEv(args), res)
	subs := decodeOverwriteSubagents(t, out)
	if len(subs) != 3 || subs[0]["Model"] != "pro" || subs[1]["Model"] != "flash_lite" || subs[2]["Model"] != "flash_lite" {
		t.Fatalf("got %v", subs)
	}
	for _, s := range subs {
		if len(s) != 2 {
			t.Fatalf("extra field written: %v", s)
		}
	}
}

func TestMultiChild_ExplicitChildPreserved(t *testing.T) {
	r := stubResolver{explicit: map[string]bool{"pro": true}}
	args := `{"Subagents":[{"Prompt":"` + mcTrivial + `","Model":"pro"},{"Prompt":"` + mcTrivial + `","Model":"flash"}]}`
	out, _, _ := antigravity.Handle(mcEv(args, "flash_lite", "flash", "pro"), r)
	subs := decodeOverwriteSubagents(t, out)
	if len(subs) != 2 || subs[0]["Model"] != "pro" || subs[1]["Model"] != "flash_lite" {
		t.Fatalf("got %v", subs)
	}
}

func TestMultiChild_AbsentModelUnconfidentIsNotWritten(t *testing.T) {
	args := `{"Subagents":[{"Prompt":"add a unit test for the parser"}]}`
	out, _, d := antigravity.Handle(mcEv(args), res)
	if out.Overwrite != nil {
		t.Fatalf("wrote %s (verdict=%v corr=%v)", out.Overwrite, d.Verdict, d.Corrections)
	}
}

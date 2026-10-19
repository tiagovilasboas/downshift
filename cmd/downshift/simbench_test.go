// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

// Offline simulation gate (internal/simbench): every scenario in
// internal/simbench/testdata/sim runs through the real shared runners
// (runHookAdapter via harnessRegistry, runHonor, runEnd) in a sandbox, and the
// accounting invariants must hold. No model call, no network. Run with -v to
// see the per-scenario ESTIMATE tables.

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/simbench"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// hookDriver binds simbench's port to the binary's shared runners.
type hookDriver struct{ t *testing.T }

func (d hookDriver) Hook(harness string, payload []byte) ([]byte, int) {
	entry, ok := harnessRegistry[harness]
	if !ok || entry.Hook == nil {
		d.t.Fatalf("no hook for %s", harness)
	}
	rc := 0
	out := captureStdout(func() {
		captureStderr(func() { rc = entry.Hook(bytes.NewReader(payload), cmdCat) })
	})
	return []byte(out), rc
}

func (d hookDriver) Honor(harness string, payload []byte) {
	captureStdout(func() {
		captureStderr(func() { runHonor(bytes.NewReader(payload), portFor(harness), cmdCat) })
	})
}

func (d hookDriver) End(harness string, payload []byte) {
	captureStdout(func() {
		captureStderr(func() { runEnd(bytes.NewReader(payload), portFor(harness), cmdCat) })
	})
}

// agentsDir is the repository's own plugin agent definitions, resolved before
// the test changes directory.
func agentsDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "..", "agents"))
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// sandbox builds a simbench environment and points the process at it.
func sandbox(t *testing.T) simbench.Env {
	t.Helper()
	env, err := simbench.NewEnv(t.TempDir(), cmdCat, agentsDir(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, kv := range os.Environ() {
		if name, _, _ := strings.Cut(kv, "="); simbench.Blank(name) {
			t.Setenv(name, "")
		}
	}
	for k, v := range env.Vars() {
		t.Setenv(k, v)
	}
	t.Chdir(env.Project)
	return env
}

func TestSimulation_AccountingInvariants(t *testing.T) {
	scenarios, err := simbench.Scenarios()
	if err != nil {
		t.Fatal(err)
	}
	if len(scenarios) == 0 {
		t.Fatal("no scenarios embedded")
	}
	var spawns, control int
	for _, sc := range scenarios {
		t.Run(sc.Name, func(t *testing.T) {
			env := sandbox(t)
			res, err := simbench.Run(sc, hookDriver{t}, env, cmdCat)
			if err != nil {
				t.Fatal(err)
			}
			var buf bytes.Buffer
			res.Format(&buf)
			for _, h := range res.Harnesses {
				spawns += h.Spawns
				control += h.Control
				buf.WriteString("  " + h.Harness + " reasons: " + h.TopReasons(4) + "\n")
			}
			t.Log("\n" + buf.String())
			for _, v := range res.Violations {
				t.Error(v)
			}
			checkCLIMatchesLibrary(t, res)
		})
	}
	// The seeded ids give a deterministic control draw near the production
	// default; a draw far from it means the share or the hash changed.
	if share := float64(control) / float64(spawns); share < 0.04 || share > 0.18 {
		t.Errorf("control share %.3f over %d spawns, want about %.2f", share, spawns, simbench.DefaultControlShare)
	}
}

// checkCLIMatchesLibrary runs `downshift report --json` and `stats --export`
// on the sandbox log and holds them to the library aggregation the invariants
// used: the CLI adds no accounting of its own.
func checkCLIMatchesLibrary(t *testing.T, res simbench.Result) {
	t.Helper()
	var out, errOut bytes.Buffer
	if rc := runReport([]string{"--days=0", "--json"}, &out, &errOut); rc != 0 {
		t.Fatalf("report rc %d: %s", rc, errOut.String())
	}
	var rep telemetry.Report
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("report --json: %v", err)
	}
	want := telemetry.BuildReport(res.Events, 0)
	if rep.Routed.Spawns != want.Routed.Spawns || rep.Control.Spawns != want.Control.Spawns ||
		rep.Routed.Completed != want.Routed.Completed || rep.Control.Completed != want.Control.Completed {
		t.Errorf("report CLI %+v / %+v, library %+v / %+v", rep.Routed, rep.Control, want.Routed, want.Control)
	}
	export := captureStdout(func() {
		if rc := runStats([]string{"--days=0", "--export"}); rc != 0 {
			t.Errorf("stats --export rc %d", rc)
		}
	})
	var sum telemetry.Summary
	if err := json.Unmarshal([]byte(export), &sum); err != nil {
		t.Fatalf("stats --export: %v\n%s", err, export)
	}
	lib := telemetry.Summarize(res.Events)
	saved := 0.0
	if sum.RealSavedUSD != nil {
		saved = *sum.RealSavedUSD
	}
	if math.Abs(saved-lib.GrossSavedUSD) > 1e-6 || math.Abs(sum.RealOverspendUSD-lib.OverspendUSD) > 1e-6 ||
		math.Abs(sum.RealCostDeltaUSD-lib.NetSavedUSD) > 1e-6 {
		t.Errorf("stats --export saved %v overspend %v net %v, Summarize %+v", saved, sum.RealOverspendUSD, sum.RealCostDeltaUSD, lib)
	}
	if strings.Contains(export, simbench.Canary) {
		t.Error("stats --export carries task text")
	}
}

// The gate must fail on each defect class it exists for. Each case plants one
// defect into a real simulated log and names the rule that has to catch it.
func TestSimulation_InvariantsCatchPlantedDefects(t *testing.T) {
	scenarios, err := simbench.Scenarios()
	if err != nil {
		t.Fatal(err)
	}
	var sc simbench.Scenario
	for _, s := range scenarios {
		if s.Name == "mechanical-renames-opus-parent" {
			sc = s
		}
	}
	if sc.Name == "" {
		t.Fatal("scenario mechanical-renames-opus-parent missing")
	}
	sc.Harnesses = []string{"claude-code"}
	// A no-signal task is held, so the wire-log case has a decision with
	// corrections to plant on.
	sc.Tasks = append(slices.Clone(sc.Tasks), "tidy the error messages in internal/invoice so they start with a lowercase letter ({canary})")
	env := sandbox(t)
	res, err := simbench.Run(sc, hookDriver{t}, env, cmdCat)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Violations) > 0 {
		t.Fatalf("clean run has violations: %v", res.Violations)
	}
	isDecision := func(ev telemetry.Event) bool {
		return !telemetry.IsObservationOutcome(ev.Outcome) && !telemetry.IsFailureOutcome(ev.Outcome)
	}
	applied := func(ev telemetry.Event) bool {
		return ev.Outcome == telemetry.OutcomeRewriteEmitted && len(ev.Corrections) == 0
	}
	control := func(ev telemetry.Event) bool {
		return ev.Outcome == telemetry.OutcomeBaseline
	}
	// usageOf returns the index of the usage record linked to the first
	// decision matching pick.
	usageOf := func(evs []telemetry.Event, pick func(telemetry.Event) bool) int {
		for _, d := range evs {
			if !isDecision(d) || !pick(d) {
				continue
			}
			for i, u := range evs {
				if u.Outcome == telemetry.OutcomeUsage && u.LinkedDecision == telemetry.DecisionKey(d) {
					return i
				}
			}
		}
		t.Fatal("no usage record for the picked decision")
		return -1
	}
	scale := func(p *float64, k float64) *float64 { v := *p * k; return &v }

	for _, tc := range []struct {
		name  string
		rule  string
		plant func(evs []telemetry.Event) []telemetry.Event
	}{
		{"saving on an unknown baseline", simbench.RuleUnknownBaseline, func(evs []telemetry.Event) []telemetry.Event {
			for i := range evs {
				if isDecision(evs[i]) && applied(evs[i]) {
					evs[i].FromModel = "unknown"
					break
				}
			}
			return evs
		}},
		{"impossible percentage (~98% class)", simbench.RulePercent, func(evs []telemetry.Event) []telemetry.Event {
			i := usageOf(evs, applied)
			evs[i].BaselineCostUSD = scale(evs[i].ActualCostUSD, 50)
			return evs
		}},
		{"saved is not baseline minus actual", simbench.RuleSavedArithmetic, func(evs []telemetry.Event) []telemetry.Event {
			i := usageOf(evs, applied)
			evs[i].ActualCostUSD = scale(evs[i].ActualCostUSD, 0.5)
			return evs
		}},
		{"control spawn recorded as routed", simbench.RuleControl, func(evs []telemetry.Event) []telemetry.Event {
			for i := range evs {
				if isDecision(evs[i]) && control(evs[i]) {
					evs[i].Outcome, evs[i].ReasonCodes = telemetry.OutcomeAllow, nil
					break
				}
			}
			return evs
		}},
		{"control end not linked", simbench.RuleControlLinked, func(evs []telemetry.Event) []telemetry.Event {
			key := ""
			for _, d := range evs {
				if isDecision(d) && control(d) {
					key = telemetry.DecisionKey(d)
					break
				}
			}
			out := evs[:0]
			for _, ev := range evs {
				if ev.Outcome == telemetry.OutcomeChildEnded && ev.LinkedDecision == key {
					continue
				}
				out = append(out, ev)
			}
			return out
		}},
		{"honored rewrite counted twice", simbench.RuleHonorOnce, func(evs []telemetry.Event) []telemetry.Event {
			for _, ev := range evs {
				if ev.Outcome == telemetry.OutcomeResolved && ev.RewriteHonored != nil {
					dup := ev
					dup.CorrelationID += "ff"
					return append(evs, dup)
				}
			}
			return evs
		}},
		{"usage priced twice", simbench.RuleOneUsage, func(evs []telemetry.Event) []telemetry.Event {
			dup := evs[usageOf(evs, applied)]
			dup.CorrelationID += "ff"
			return append(evs, dup)
		}},
		{"saving quoted for a rewrite the hook never wrote", simbench.RuleWireLog, func(evs []telemetry.Event) []telemetry.Event {
			i := usageOf(evs, func(ev telemetry.Event) bool { return len(ev.Corrections) > 0 })
			for j := range evs {
				if isDecision(evs[j]) && telemetry.DecisionKey(evs[j]) == evs[i].LinkedDecision {
					evs[j].Outcome, evs[j].Corrections = telemetry.OutcomeRewriteEmitted, nil
				}
			}
			evs[i].ActualCostUSD = scale(evs[i].BaselineCostUSD, 0.4)
			return evs
		}},
		{"child tokens miscounted", simbench.RuleTokens, func(evs []telemetry.Event) []telemetry.Event {
			i := usageOf(evs, applied)
			n := *evs[i].InputTokens * 3
			evs[i].InputTokens = &n
			return evs
		}},
		{"one spawn logged twice", simbench.RuleOneDecision, func(evs []telemetry.Event) []telemetry.Event {
			for _, ev := range evs {
				if isDecision(ev) {
					return append(evs, ev)
				}
			}
			return evs
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			planted := tc.plant(slices.Clone(res.Events))
			if got := rulesOf(simbench.Check(sc, res.Traces, planted, cmdCat)); !got[tc.rule] {
				t.Errorf("planted %q, rule %s did not fire (fired: %v)", tc.name, tc.rule, got)
			}
		})
	}

	t.Run("task text on disk", func(t *testing.T) {
		leak := filepath.Join(env.StateDir, "leak.json")
		if err := os.WriteFile(leak, []byte(`{"note":"`+res.Traces[0].Task+`"}`), 0o600); err != nil {
			t.Fatal(err)
		}
		v, err := simbench.ScanPromptFree(res.Traces, env.StateDir)
		if err != nil {
			t.Fatal(err)
		}
		if !rulesOf(v)[simbench.RulePromptFree] {
			t.Error("a file holding task text passed the prompt-free scan")
		}
	})
}

func rulesOf(vs []simbench.Violation) map[string]bool {
	out := map[string]bool{}
	for _, v := range vs {
		out[v.Rule] = true
	}
	return out
}

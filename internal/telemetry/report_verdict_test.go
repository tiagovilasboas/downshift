// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"fmt"
	"strings"
	"testing"
)

// noted sets the routing note of the decision with this correlation id.
func (f *reportFixture) noted(id, verdict, reason string) {
	for i := range f.events {
		if f.events[i].CorrelationID == id {
			f.events[i].RoutingNote = &RoutingNote{Verdict: verdict, Reason: reason}
		}
	}
}

// redone adds n routed spawns of one verdict, each a failed attempt that a
// completed spawn of the same session redid a minute later.
func (f *reportFixture) redone(verdict string, n int) []string {
	var failed []string
	for i := 0; i < n; i++ {
		sess := fmt.Sprintf("%s-redo-%d", verdict, i)
		id := f.spawn(GroupRouted, 0.1, ChildError, sess)
		f.noted(id, verdict, "")
		f.noted(f.spawn(GroupRouted, 0.1, ChildCompleted, sess), verdict, "")
		failed = append(failed, id)
	}
	return failed
}

// clean adds n routed spawns of one verdict that completed with no rework.
func (f *reportFixture) clean(verdict, reason string, n int) {
	for i := 0; i < n; i++ {
		f.noted(f.spawn(GroupRouted, 0.1, ChildCompleted, fmt.Sprintf("%s-%s-%d", verdict, reason, i)), verdict, reason)
	}
}

func verdictRow(t *testing.T, rep Report, verdict string) VerdictRework {
	t.Helper()
	for _, v := range rep.ByVerdict {
		if v.Verdict == verdict {
			return v
		}
	}
	t.Fatalf("no row for verdict %q in %+v", verdict, rep.ByVerdict)
	return VerdictRework{}
}

func TestBuildReport_ReworkByVerdictRows(t *testing.T) {
	f := newFixture()
	failed := f.redone("class", 25)
	f.clean("rule", "", 25)
	f.redone("hint_down", 5)
	rep := BuildReport(f.events, 30)

	class := verdictRow(t, rep, "class")
	if class.Completed != 25 || class.Reworked != 25 || class.Ended != 50 || class.ReworkRate != 0.5 {
		t.Fatalf("class = %+v", class)
	}
	if class.TurnOff != "routing.by_class: false" {
		t.Errorf("turn off key = %q", class.TurnOff)
	}
	if strings.Join(class.ReworkedCorrelationIDs, ",") != strings.Join(failed, ",") {
		t.Errorf("reworked ids = %v, want the failed decisions %v", class.ReworkedCorrelationIDs, failed)
	}
	if rule := verdictRow(t, rep, "rule"); rule.Reworked != 0 || len(rule.ReworkedCorrelationIDs) != 0 || rule.TurnOff != "routing.rules[].enabled: false" {
		t.Errorf("rule has no rework and keeps its key: %+v", rule)
	}
	if hint := verdictRow(t, rep, "hint_down"); hint.Completed != 5 || hint.TurnOff != "" {
		t.Errorf("hint_down has no key to turn it off: %+v", hint)
	}
}

// Paused spawns are not routed: they enter no verdict row.
func TestBuildReport_ReworkByVerdictLeavesUnroutedOut(t *testing.T) {
	f := newFixture()
	f.clean("class", "", 2)
	f.spawn(GroupUnrouted, 0.2, ChildCompleted, "paused")
	rep := BuildReport(f.events, 30)
	if got := verdictRow(t, rep, "class").Spawns; got != 2 {
		t.Errorf("class spawns = %d, want 2", got)
	}
	for _, v := range rep.ByVerdict {
		if v.Verdict == VerdictNotRecorded {
			t.Errorf("a paused spawn must not be a not_recorded routed row: %+v", v)
		}
	}
}

func TestBuildReport_VerdictLabels(t *testing.T) {
	f := newFixture()
	f.clean("kept", "held_after_failure", 1)
	f.clean("kept", "", 1)
	f.spawn(GroupRouted, 0.1, ChildCompleted, "no-note")
	f.noted(f.spawn(GroupRouted, 0.1, ChildCompleted, "free-text"), "a prompt with spaces", "")
	rep := BuildReport(f.events, 30)
	for _, want := range []string{"held_after_failure", "kept", VerdictNotRecorded} {
		verdictRow(t, rep, want)
	}
	if got := verdictRow(t, rep, VerdictNotRecorded).Spawns; got != 2 {
		t.Errorf("a missing note and a free-text verdict both count as not recorded, got %d", got)
	}
}

func TestVerdictSwitchNamesTheKeyOfEveryVerdictThatHasOne(t *testing.T) {
	want := map[string]string{
		"class": "routing.by_class: false", "class_up": "routing.raise_complex: false",
		"rule": "routing.rules[].enabled: false", "agent_default": "routing.agent_default_tier: false",
	}
	for verdict, key := range want {
		if verdictSwitch[verdict] != key {
			t.Errorf("%s -> %q, want %q", verdict, verdictSwitch[verdict], key)
		}
	}
	for _, none := range []string{"kept", "hint_down", "held_unknown", "refused_config", "default", "held_after_failure", VerdictNotRecorded} {
		if k, ok := verdictSwitch[none]; ok {
			t.Errorf("%s has no policy key but maps to %q", none, k)
		}
	}
}

func TestPrintReport_ReworkByVerdictHasNoControlColumnOrFlag(t *testing.T) {
	f := newFixture()
	f.redone("class", 25)
	f.redone("hint_down", 3)
	f.clean("rule", "", 20)
	var b strings.Builder
	PrintReport(&b, BuildReport(f.events, 30))
	out := b.String()
	for _, want := range []string{
		"Rework by routing verdict, routed spawns\n",
		"policy key that turns it off",
		"routing.by_class: false",
		"routing.rules[].enabled: false",
		"No policy key turns this verdict off",
		"Downshift does not change the policy",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("report lacks %q:\n%s", want, out)
		}
	}
	for _, gone := range []string{"control", "against control", "worse", "not enough data"} {
		if strings.Contains(out, gone) {
			t.Errorf("report must not carry %q:\n%s", gone, out)
		}
	}
}

func TestReportJSON_ReworkByVerdictListsCorrelationIDs(t *testing.T) {
	f := newFixture()
	failed := f.redone("class", 2)
	raw, err := ReportJSON(BuildReport(f.events, 30))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range append([]string{`"rework_by_verdict"`, `"reworked_correlation_ids"`}, failed...) {
		if !strings.Contains(raw, want) {
			t.Errorf("json lacks %s:\n%s", want, raw)
		}
	}
}

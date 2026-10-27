// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package contract

import (
	"reflect"
	"testing"
	"time"
)

var timeType = reflect.TypeOf(time.Time{})

// freeTextFields returns the fields of struct type t that could carry free
// text: a bare string, or any type other than a named string declared in
// this package, a number, a bool or a time. Pure, so the negative test can
// prove it catches a violation.
func freeTextFields(t reflect.Type) []string {
	pkg := reflect.TypeOf(Note{}).PkgPath()
	var bad []string
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		ft := f.Type
		switch {
		case ft == timeType:
		case ft.Kind() == reflect.Bool:
		case ft.Kind() >= reflect.Int && ft.Kind() <= reflect.Float64:
		case ft.Kind() == reflect.String && ft.PkgPath() == pkg && ft.Name() != "":
		default:
			bad = append(bad, f.Name)
		}
	}
	return bad
}

func TestNoteHasNoFreeTextField(t *testing.T) {
	if bad := freeTextFields(reflect.TypeOf(Note{})); len(bad) > 0 {
		t.Fatalf("Note fields %v are not enum, number, bool or time; the routing note must never carry task text or paths", bad)
	}
}

func TestFreeTextFieldsDetectsViolations(t *testing.T) {
	type leaky struct {
		Verdict Verdict
		Task    string
		Paths   []string
		Meta    map[string]Tier
		Inner   struct{ X int }
		Ptr     *Tier
		At      time.Time
		Pct     float64
	}
	got := freeTextFields(reflect.TypeOf(leaky{}))
	want := []string{"Task", "Paths", "Meta", "Inner", "Ptr"}
	if len(got) != len(want) {
		t.Fatalf("freeTextFields = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("freeTextFields = %v, want %v", got, want)
		}
	}
}

// Request carries ids and enums only. Pinning its field set makes adding a
// field (for example the task text) a reviewed contract change.
func TestRequestFieldSetIsPinned(t *testing.T) {
	want := []string{"Harness", "RequestedModel", "Pinned", "AgentType", "Profile", "Hint"}
	rt := reflect.TypeOf(Request{})
	if rt.NumField() != len(want) {
		t.Fatalf("Request has %d fields, want %v; task text never enters routing", rt.NumField(), want)
	}
	for i, n := range want {
		if rt.Field(i).Name != n {
			t.Errorf("Request field %d = %s, want %s", i, rt.Field(i).Name, n)
		}
	}
	if bad := freeTextFields(reflect.TypeOf(Hint{})); len(bad) > 0 {
		t.Errorf("Hint fields %v are not enums", bad)
	}
}

func TestZeroReadingIsUnknown(t *testing.T) {
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	if (Reading{}).Fresh(now) {
		t.Fatal("zero Reading is fresh; it must be unknown")
	}
	ok := Reading{Window: WindowWeek, UsedPct: 60, ObservedAt: now.Add(-time.Minute), TTL: 5 * time.Minute}
	cases := []struct {
		name string
		r    Reading
		want bool
	}{
		{"fresh", ok, true},
		{"at ttl edge", with(ok, func(r *Reading) { r.ObservedAt = now.Add(-5 * time.Minute) }), true},
		{"past ttl is unknown (REA divergence)", with(ok, func(r *Reading) { r.ObservedAt = now.Add(-6 * time.Minute) }), false},
		{"no ttl is unknown", with(ok, func(r *Reading) { r.TTL = 0 }), false},
		{"future observation", with(ok, func(r *Reading) { r.ObservedAt = now.Add(time.Minute) }), false},
		{"pct over 100", with(ok, func(r *Reading) { r.UsedPct = 101 }), false},
		{"negative pct", with(ok, func(r *Reading) { r.UsedPct = -1 }), false},
		{"rate limited stays fresh", with(ok, func(r *Reading) { r.RateLimited = true; r.UsedPct = 0 }), true},
	}
	for _, c := range cases {
		if got := c.r.Fresh(now); got != c.want {
			t.Errorf("%s: Fresh = %v, want %v", c.name, got, c.want)
		}
	}
}

func with(r Reading, f func(*Reading)) Reading { f(&r); return r }

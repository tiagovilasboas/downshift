// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestParseMark(t *testing.T) {
	cases := []struct {
		in   string
		want Mark
		ok   bool
	}{{"", MarkNone, true}, {"used", MarkUsed, true}, {"discarded", MarkDiscarded, true}, {"Used", MarkNone, false}, {"kept", MarkNone, false}}
	for _, c := range cases {
		if got, ok := ParseMark(c.in); got != c.want || ok != c.ok {
			t.Fatalf("ParseMark(%q) = %q,%v", c.in, got, ok)
		}
	}
}

func TestReportKeepsSizesTokensAndTheMarkPastRelease(t *testing.T) {
	l := newResumeLaunch()
	q := New(l, 4, fixedNow)
	secret := "do-not-persist-this-task-text"
	if _, _, err := q.DelegateWave([]Spec{spec(secret, time.Minute), spec("quiet", time.Minute)}, WaveLimits{}); err != nil {
		t.Fatal(err)
	}
	p := l.proc(secret)
	p.tokens = &Tokens{Total: 300}
	p.finish("twelve bytes", nil)
	l.proc("quiet").finish("", nil) // no usage reported
	waitFor(t, "both reports", func() bool { return len(q.Reports()) == 2 })

	got := q.Reports()
	a, b := got[0], got[1]
	if a.Job != "w001" || a.Agent != "claude" || a.Wave != 1 || a.State != Done || a.ResultBytes != 12 || a.Tokens != 300 || !a.TranscriptOnDisk || a.Status != StatusUnmarked {
		t.Fatalf("report a = %+v", a)
	}
	if b.Tokens != 0 || b.ResultBytes != 0 {
		t.Fatalf("report b = %+v, want no tokens and no result", b)
	}

	if rel, _, refused := q.ReleaseMarked([]string{"w001"}, MarkUsed); len(rel) != 1 || len(refused) != 0 {
		t.Fatalf("released %v refused %v", rel, refused)
	}
	if rel, _, _ := q.ReleaseMarked([]string{"w002"}, MarkNone); len(rel) != 1 {
		t.Fatalf("released %v", rel)
	}
	got = q.Reports()
	if len(got) != 2 || got[0].Status != "used" || got[0].TranscriptOnDisk || got[1].Status != StatusUnmarked {
		t.Fatalf("after release = %+v", got)
	}
	raw, _ := json.Marshal(got)
	if strings.Contains(string(raw), secret) || strings.Contains(string(raw), "twelve") {
		t.Fatalf("a report carries task or result text: %s", raw)
	}
}

func TestReportSumsFollowUpTurnsAndTheCancelledHoldsNoTranscript(t *testing.T) {
	l := newResumeLaunch()
	q := New(l, 4, fixedNow)
	if _, err := q.Delegate([]Spec{spec("a", time.Minute), spec("b", time.Minute)}); err != nil {
		t.Fatal(err)
	}
	l.proc("a").tokens = &Tokens{Total: 100}
	l.proc("a").finish("first", nil)
	waitFor(t, "first turn", func() bool { return q.TokensUsed("claude") == 100 })
	if err := q.Continue("w001", "next"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "resumed", func() bool { return l.proc("next") != nil })
	l.proc("next").tokens = &Tokens{Total: 50}
	l.proc("next").finish("second turn", nil)
	waitFor(t, "second turn", func() bool { return q.TokensUsed("claude") == 150 })
	q.Cancel([]string{"w002"})

	got := q.Reports()
	if got[0].Tokens != 150 || got[0].ResultBytes != len("second turn") {
		t.Fatalf("report a = %+v", got[0])
	}
	if got[1].State != Cancelled || got[1].TranscriptOnDisk {
		t.Fatalf("report b = %+v", got[1])
	}
}

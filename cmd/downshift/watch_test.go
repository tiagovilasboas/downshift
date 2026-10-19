// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

var watchT0 = time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)

func watchEv(id, sess string, d time.Duration, outcome string) telemetry.Event {
	return telemetry.Event{
		CorrelationID: id, Timestamp: watchT0.Add(d).Format(time.RFC3339Nano), SessionID: sess,
		Harness: "claude-code", Outcome: outcome, Complexity: "MEDIUM",
		FromModel: "claude-opus-5-5", ToModel: "claude-sonnet-5-5", Tier: "mid", Verdict: "DOWNSHIFT",
	}
}

func TestWatchFrame_FakeClockAndTimeoutToUnknown(t *testing.T) {
	t.Setenv("DOWNSHIFT_STATE_DIR", t.TempDir())
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	sess := telemetry.HashSessionID("w-sess")
	events := []telemetry.Event{
		watchEv("d1", sess, 0, telemetry.OutcomeRewriteEmitted),
		watchEv("d2", sess, 100*time.Minute, telemetry.OutcomeRewriteEmitted),
	}

	// 1h50 after d1, 10 min after d2: both still running.
	out, err := watchFrame(events, "", cmdCat, watchT0.Add(110*time.Minute), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "downshift watch · 10:50:00 · every 1s · Ctrl-C to quit\n○ downshift idle") {
		t.Fatalf("frame header:\n%s", out)
	}
	if strings.Count(out, "  …  ") != 2 || strings.Contains(out, "  ?  ") {
		t.Fatalf("both children should be running:\n%s", out)
	}

	// 2h05 after d1: the first runs out to ?, the second is still running.
	out, _ = watchFrame(events, "", cmdCat, watchT0.Add(125*time.Minute), time.Second)
	rows := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(rows) != 5 || !strings.Contains(rows[3], "  ?  ") || !strings.Contains(rows[4], "  …  ") {
		t.Fatalf("d1 must be ?, d2 running:\n%s", out)
	}
}

func TestWatchFrame_NoSessionFallback(t *testing.T) {
	t.Setenv("DOWNSHIFT_STATE_DIR", t.TempDir())
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	out, err := watchFrame(nil, "", cmdCat, watchT0, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "session · no subagents yet") || !strings.Contains(out, "waiting for a session") {
		t.Fatalf("empty log:\n%s", out)
	}
	if _, err := watchFrame(nil, "bad id with spaces", cmdCat, watchT0, time.Second); err == nil {
		t.Fatal("an invalid explicit session id must be an error")
	}
}

func TestWatchRun_LoopClearsAndStopsOnCancel(t *testing.T) {
	t.Setenv("DOWNSHIFT_STATE_DIR", t.TempDir())
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	ctx, cancel := context.WithCancel(context.Background())
	frames := 0
	read := func() []telemetry.Event {
		frames++
		if frames >= 3 {
			cancel()
		}
		return nil
	}
	var o, e bytes.Buffer
	rc := watchRun(ctx, []string{"--interval=1ms"}, &o, &e, cmdCat, func() time.Time { return watchT0 }, read)
	if rc != 0 || e.Len() != 0 {
		t.Fatalf("rc=%d stderr=%q", rc, e.String())
	}
	if n := strings.Count(o.String(), ansiClear); n < 2 {
		t.Fatalf("expected the screen cleared per frame, got %d", n)
	}
}

func TestWatchRun_OnceHasNoAnsiAndRejectsBadArgs(t *testing.T) {
	t.Setenv("DOWNSHIFT_STATE_DIR", t.TempDir())
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	var o, e bytes.Buffer
	if rc := watchRun(context.Background(), []string{"--once"}, &o, &e, cmdCat, func() time.Time { return watchT0 }, func() []telemetry.Event { return nil }); rc != 0 || strings.Contains(o.String(), "\x1b") {
		t.Fatalf("once: rc=%d %q", rc, o.String())
	}
	for _, args := range [][]string{{"extra"}, {"--once", "--session=bad id"}, {"--nope"}} {
		if rc := watchRun(context.Background(), args, &o, &e, cmdCat, time.Now, func() []telemetry.Event { return nil }); rc != 2 {
			t.Fatalf("%v: rc=%d", args, rc)
		}
	}
}

func TestWatch_RealLogOnceFrame(t *testing.T) {
	seedTreeLog(t)
	t.Setenv("DOWNSHIFT_STATE_DIR", t.TempDir())
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	var o, e bytes.Buffer
	rc := watchRun(context.Background(), []string{"--once", "--session=sess-a"}, &o, &e, cmdCat, time.Now, func() []telemetry.Event {
		ev, _ := telemetry.ReadEventsTail(telemetry.DefaultTailBytes)
		return ev
	})
	if rc != 0 || !strings.Contains(o.String(), "saved $0.38") {
		t.Fatalf("rc=%d\n%s", rc, o.String())
	}
}

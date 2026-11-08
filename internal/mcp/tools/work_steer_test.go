// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/workers"
)

// The steer tool corrects a worker mid-turn without losing its conversation. It
// calls Queue.Steer: a live turn is interrupted and the correction resumes the
// same session first; an idle worker takes the correction at the front of its
// queue. These tests reuse the send tool's controllable launcher and queue (no
// CLI) and are unhappy-path first (regra 1:3): the schema and error mappings
// come before the two happy paths and the concurrency guarantee.

// resumeTaskAt reads the task of the nth recorded resume under the launcher's
// lock, so a test never races the LaunchResume goroutine that appends it. It
// returns "" when there is no nth resume yet.
func resumeTaskAt(l *sendLauncher, n int) string {
	l.mu.Lock()
	defer l.mu.Unlock()
	if n < 0 || n >= len(l.resumes) {
		return ""
	}
	return l.resumes[n].Task
}

// steerTool returns the steer tool from the work set.
func steerTool(t *testing.T, d Deps) Tool {
	t.Helper()
	for _, tl := range WorkTools(d) {
		if tl.Name == "steer" {
			return tl
		}
	}
	t.Fatal("no steer tool")
	return Tool{}
}

// callSteer runs steer and decodes its data map (or returns the error).
func callSteer(t *testing.T, d Deps, args string) (map[string]any, error) {
	t.Helper()
	res, err := steerTool(t, d).Handler(context.Background(), json.RawMessage(args))
	if err != nil {
		return nil, err
	}
	var env struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal([]byte(res.Text), &env); err != nil {
		t.Fatalf("envelope is not JSON: %v", err)
	}
	return env.Data, nil
}

// --- unhappy: schema and the queue error mappings ---

// TestSteerRefusesMissingArguments covers the schema guards before the queue is
// reached: no job_id, no message, an unknown field.
func TestSteerRefusesMissingArguments(t *testing.T) {
	d, _ := sendDeps(t)
	cases := []struct {
		name string
		args string
		want string
	}{
		{"no job_id", `{"message":"fix"}`, "job_id"},
		{"blank job_id", `{"job_id":"   ","message":"fix"}`, "job_id"},
		{"no message", `{"job_id":"w001"}`, "message"},
		{"unknown field", `{"job_id":"w001","message":"x","foo":1}`, "do not match"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := callSteer(t, d, c.args); err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want one mentioning %q", err, c.want)
			}
		})
	}
}

// TestSteerWithoutAQueueSaysSo: a server with no worker queue answers that
// workers are unavailable, like the other work tools.
func TestSteerWithoutAQueueSaysSo(t *testing.T) {
	d := Deps{Strain: fakeStrain{}}
	if _, err := callSteer(t, d, `{"job_id":"w001","message":"fix"}`); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("err = %v, want the no-queue message", err)
	}
}

// TestSteerMapsEachQueueErrorToAClearMessage drives Steer into each of its
// errors and checks steer reports a clear, task-free message for each.
func TestSteerMapsEachQueueErrorToAClearMessage(t *testing.T) {
	// ErrUnknownJob: no worker with that id.
	t.Run("unknown job", func(t *testing.T) {
		d, _ := sendDeps(t)
		if _, err := callSteer(t, d, `{"job_id":"w404","message":"fix"}`); err == nil ||
			!strings.Contains(err.Error(), "no worker with that id") {
			t.Fatalf("err = %v, want the unknown-job message", err)
		}
	})

	// ErrNotSteerable: a cancelled worker cannot be steered.
	t.Run("not steerable", func(t *testing.T) {
		d, _ := sendDeps(t)
		id := delegateOne(t, d, "run")
		d.Work.Cancel([]string{id})
		if _, err := callSteer(t, d, `{"job_id":"`+id+`","message":"fix"}`); err == nil ||
			!strings.Contains(err.Error(), "cannot be steered") {
			t.Fatalf("err = %v, want the not-steerable message", err)
		}
	})

	// ErrEmptyTask: a whitespace message is refused by the queue (the schema
	// allows a non-empty string, so the trim happens in Steer).
	t.Run("empty message", func(t *testing.T) {
		d, _ := sendDeps(t)
		id := delegateOne(t, d, "run")
		waitState(t, d, id, workers.Running)
		if _, err := callSteer(t, d, `{"job_id":"`+id+`","message":" "}`); err == nil ||
			!strings.Contains(err.Error(), "needs text") {
			t.Fatalf("err = %v, want the empty-message message", err)
		}
	})

	// ErrTaskTooLong: a message over the byte cap.
	t.Run("message too long", func(t *testing.T) {
		d, _ := sendDeps(t)
		id := delegateOne(t, d, "run")
		waitState(t, d, id, workers.Running)
		big := strings.Repeat("x", workers.MaxTaskBytes+1)
		if _, err := callSteer(t, d, `{"job_id":"`+id+`","message":"`+big+`"}`); err == nil ||
			!strings.Contains(err.Error(), "size limit") {
			t.Fatalf("err = %v, want the too-long message", err)
		}
	})

	// ErrInboxFull: the worker already holds the most queued turns it may.
	t.Run("inbox full", func(t *testing.T) {
		d, _ := sendDeps(t)
		id := delegateOne(t, d, "run")
		waitState(t, d, id, workers.Running)
		for i := 0; i < workers.MaxInbox; i++ {
			if _, err := callSend(t, d, `{"job_id":"`+id+`","task":"turn"}`); err != nil {
				t.Fatalf("enqueue %d: %v", i, err)
			}
		}
		if _, err := callSteer(t, d, `{"job_id":"`+id+`","message":"one too many"}`); err == nil ||
			!strings.Contains(err.Error(), "most queued turns") {
			t.Fatalf("err = %v, want the inbox-full message", err)
		}
	})
}

// --- happy: interrupt a live turn, degrade to a front send when idle ---

// TestSteerInterruptsALiveTurnAndReportsSteering is the core path: a steer on a
// running worker interrupts the live turn and reports "steering" with position
// 0 (the correction runs now, ahead of any queued turn), and the queue resumes
// once with the correction as the first task.
func TestSteerInterruptsALiveTurnAndReportsSteering(t *testing.T) {
	d, l := sendDeps(t)
	id := delegateOne(t, d, "turn-1")
	waitState(t, d, id, workers.Running)
	// A send had already queued a later turn; the steer must run before it.
	if _, err := callSend(t, d, `{"job_id":"`+id+`","task":"queued-before"}`); err != nil {
		t.Fatalf("send: %v", err)
	}

	data, err := callSteer(t, d, `{"job_id":"`+id+`","message":"correction"}`)
	if err != nil {
		t.Fatalf("steer: %v", err)
	}
	if data["state"] != "steering" {
		t.Errorf("state = %v, want steering", data["state"])
	}
	if pos, _ := data["position"].(float64); pos != 0 {
		t.Errorf("position = %v, want 0 for a steer that runs now", data["position"])
	}
	// The interrupted turn settled resumable and the correction resumed first.
	deadline := time.Now().Add(2 * time.Second)
	for l.resumeCount() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if l.resumeCount() != 1 {
		t.Fatalf("the correction did not resume after the interrupt")
	}
	if got := resumeTaskAt(l, 0); got != "correction" {
		t.Errorf("first resumed task = %q, want correction (ahead of the queued send)", got)
	}
}

// TestSteerOnASettledWorkerResumesNow is the idle degrade: with no live turn a
// steer resumes the session now with the correction, reported as steering.
func TestSteerOnASettledWorkerResumesNow(t *testing.T) {
	d, l := sendDeps(t)
	id := delegateOne(t, d, "turn-1")
	waitState(t, d, id, workers.Running)
	l.proc("turn-1").finish("first", nil)
	waitState(t, d, id, workers.Done)

	data, err := callSteer(t, d, `{"job_id":"`+id+`","message":"correction"}`)
	if err != nil {
		t.Fatalf("steer: %v", err)
	}
	if data["state"] != "steering" {
		t.Errorf("state = %v, want steering", data["state"])
	}
	waitState(t, d, id, workers.Running)
	if l.resumeCount() != 1 || resumeTaskAt(l, 0) != "correction" {
		t.Fatalf("resume count = %d / first task = %q, want one with the correction", l.resumeCount(), resumeTaskAt(l, 0))
	}
}

// --- concurrency: steer does not block collect, cancel or status ---

// TestSteerDoesNotBlockCollectCancelStatus is the regra-2 guarantee: while a
// steer interrupts and the session resumes, collect, cancel and status all
// answer at once, never serialised behind the turn.
func TestSteerDoesNotBlockCollectCancelStatus(t *testing.T) {
	d, _ := sendDeps(t)
	id := delegateOne(t, d, "turn-1")
	waitState(t, d, id, workers.Running)
	if _, err := callSteer(t, d, `{"job_id":"`+id+`","message":"correction"}`); err != nil {
		t.Fatalf("steer: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = d.Work.Workers()
		_ = d.Work.Collect(context.Background(), 0, false, 0)
		_ = d.Work.Cancel([]string{id})
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("collect/cancel/status blocked behind a steer")
	}
}

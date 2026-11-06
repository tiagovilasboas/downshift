// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/workers"
)

// The send tool hands more work to an existing worker on the same conversation.
// It calls Queue.Continue: a live worker takes the turn in its inbox and runs
// it next; a settled, resumable worker is resumed now. These tests drive a real
// queue over a controllable launcher, never a CLI, and are unhappy-path first
// (regra 1:3): the error mappings come before the two happy paths and the
// concurrency guarantee.

// sendProc is a worker a test ends by hand. It reports a session id so a
// settled worker is resumable, and signals when it is killed so a cancel test
// can see it. A turn blocks in Wait until the test calls finish.
type sendProc struct {
	mu       sync.Mutex
	done     chan struct{}
	killedCh chan struct{}
	report   string
	session  string
	waitErr  error
}

func newSendProc(session string) *sendProc {
	return &sendProc{done: make(chan struct{}), killedCh: make(chan struct{}, 1), session: session}
}

func (p *sendProc) Wait() error {
	<-p.done
	return p.waitErr
}

// finish ends the current turn with a report (and optional error), so the queue
// settles the worker and drains its inbox.
func (p *sendProc) finish(report string, err error) {
	p.mu.Lock()
	p.report, p.waitErr = report, err
	p.mu.Unlock()
	close(p.done)
}

func (p *sendProc) Kill() error {
	select {
	case p.killedCh <- struct{}{}:
	default:
	}
	// A killed turn's Wait must return, so the queue can settle it.
	select {
	case <-p.done:
	default:
		close(p.done)
	}
	return nil
}

func (p *sendProc) Report() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.report
}
func (p *sendProc) Stderr() string          { return "" }
func (p *sendProc) Tokens() *workers.Tokens { return nil }
func (p *sendProc) SessionID() string       { return p.session }

// sendLauncher is a controllable launcher with resume. Every Launch and every
// LaunchResume hands back a fresh sendProc keyed by its task text, so a test can
// reach the right turn and end it. It records each resume so a test can assert
// the queue resumed exactly once, with the original spec.
type sendLauncher struct {
	mu      sync.Mutex
	procs   map[string]*sendProc
	resumes []workers.Spec
	session string // the session id each first turn reports
}

func newSendLauncher() *sendLauncher {
	return &sendLauncher{procs: map[string]*sendProc{}, session: "sess-1"}
}

func (l *sendLauncher) Launch(s workers.Spec) (workers.Process, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	p := newSendProc(l.session)
	l.procs[s.Task] = p
	return p, nil
}

func (l *sendLauncher) LaunchResume(s workers.Spec, sessionID string) (workers.Process, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	p := newSendProc(sessionID)
	l.procs[s.Task] = p
	l.resumes = append(l.resumes, s)
	return p, nil
}

func (l *sendLauncher) proc(task string) *sendProc {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.procs[task]
}

func (l *sendLauncher) resumeCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.resumes)
}

// sendDeps wires a real queue over a resumable launcher. limit 1 keeps one turn
// at a time, so a second task waits in the inbox while the first runs.
func sendDeps(t *testing.T) (Deps, *sendLauncher) {
	t.Helper()
	l := newSendLauncher()
	d := Deps{
		Strain:    fakeStrain{},
		Harnesses: func() []string { return []string{"claude-code", "codex"} },
		Work:      workers.New(l, 1, time.Now),
		Root:      "/proj",
	}
	t.Cleanup(d.Work.Close)
	return d, l
}

// sendTool returns the send tool from the work set.
func sendTool(t *testing.T, d Deps) Tool {
	t.Helper()
	for _, tl := range WorkTools(d) {
		if tl.Name == "send" {
			return tl
		}
	}
	t.Fatal("no send tool")
	return Tool{}
}

// callSend runs send and decodes its data map (or returns the error).
func callSend(t *testing.T, d Deps, args string) (map[string]any, error) {
	t.Helper()
	res, err := sendTool(t, d).Handler(context.Background(), json.RawMessage(args))
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

// delegateOne starts one read worker and returns its id, so a send test has a
// worker to continue.
func delegateOne(t *testing.T, d Deps, task string) string {
	t.Helper()
	res, err := d.delegate(context.Background(), json.RawMessage(
		`{"tasks":["`+task+`"],"agent":"claude","mode":"read"}`))
	if err != nil {
		t.Fatalf("delegate: %v", err)
	}
	var env struct {
		Data struct {
			Jobs []struct {
				ID string `json:"id"`
			} `json:"jobs"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(res.Text), &env); err != nil {
		t.Fatalf("delegate envelope: %v", err)
	}
	if len(env.Data.Jobs) != 1 {
		t.Fatalf("delegate returned %+v, want one job", env.Data.Jobs)
	}
	return env.Data.Jobs[0].ID
}

func waitState(t *testing.T, d Deps, id string, want workers.State) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, v := range d.Work.Workers() {
			if v.ID == id && v.State == want {
				return
			}
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("worker %s did not reach state %s", id, want)
}

// --- unhappy: schema and the queue error mappings ---

// TestSendRefusesMissingArguments covers the schema guards before the queue is
// even reached: no job_id, no task, an unknown field.
func TestSendRefusesMissingArguments(t *testing.T) {
	d, _ := sendDeps(t)
	cases := []struct {
		name string
		args string
		want string
	}{
		{"no job_id", `{"task":"more"}`, "job_id"},
		{"blank job_id", `{"job_id":"   ","task":"more"}`, "job_id"},
		{"no task", `{"job_id":"w001"}`, "task"},
		{"unknown field", `{"job_id":"w001","task":"x","foo":1}`, "do not match"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := callSend(t, d, c.args); err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want one mentioning %q", err, c.want)
			}
		})
	}
}

// TestSendWithoutAQueueSaysSo: a server with no worker queue answers that
// workers are unavailable, like the other work tools.
func TestSendWithoutAQueueSaysSo(t *testing.T) {
	d := Deps{Strain: fakeStrain{}}
	if _, err := callSend(t, d, `{"job_id":"w001","task":"more"}`); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("err = %v, want the no-queue message", err)
	}
}

// TestSendMapsEachQueueErrorToAClearMessage drives Continue into each of its
// errors and checks send reports a clear, task-free message for each.
func TestSendMapsEachQueueErrorToAClearMessage(t *testing.T) {
	// ErrUnknownJob: no worker with that id.
	t.Run("unknown job", func(t *testing.T) {
		d, _ := sendDeps(t)
		if _, err := callSend(t, d, `{"job_id":"w404","task":"more"}`); err == nil ||
			!strings.Contains(err.Error(), "no worker with that id") {
			t.Fatalf("err = %v, want the unknown-job message", err)
		}
	})

	// ErrNotResumable: a cancelled worker cannot take another turn.
	t.Run("not resumable", func(t *testing.T) {
		d, _ := sendDeps(t)
		id := delegateOne(t, d, "run")
		d.Work.Cancel([]string{id})
		if _, err := callSend(t, d, `{"job_id":"`+id+`","task":"more"}`); err == nil ||
			!strings.Contains(err.Error(), "cannot take another turn") {
			t.Fatalf("err = %v, want the not-resumable message", err)
		}
	})

	// ErrEmptyTask: a whitespace task is refused by the queue (the schema allows
	// a non-empty string, so the trim happens in Continue).
	t.Run("empty task", func(t *testing.T) {
		d, _ := sendDeps(t)
		id := delegateOne(t, d, "run")
		waitState(t, d, id, workers.Running)
		if _, err := callSend(t, d, `{"job_id":"`+id+`","task":" "}`); err == nil ||
			!strings.Contains(err.Error(), "needs a task") {
			t.Fatalf("err = %v, want the empty-task message", err)
		}
	})

	// ErrTaskTooLong: a turn over the byte cap.
	t.Run("task too long", func(t *testing.T) {
		d, _ := sendDeps(t)
		id := delegateOne(t, d, "run")
		waitState(t, d, id, workers.Running)
		big := strings.Repeat("x", workers.MaxTaskBytes+1)
		if _, err := callSend(t, d, `{"job_id":"`+id+`","task":"`+big+`"}`); err == nil ||
			!strings.Contains(err.Error(), "size limit") {
			t.Fatalf("err = %v, want the too-long message", err)
		}
	})

	// ErrInboxFull: the worker already holds the most queued turns it may.
	t.Run("inbox full", func(t *testing.T) {
		d, l := sendDeps(t)
		id := delegateOne(t, d, "run")
		waitState(t, d, id, workers.Running)
		for i := 0; i < workers.MaxInbox; i++ {
			if _, err := callSend(t, d, `{"job_id":"`+id+`","task":"turn"}`); err != nil {
				t.Fatalf("enqueue %d: %v", i, err)
			}
		}
		if _, err := callSend(t, d, `{"job_id":"`+id+`","task":"one too many"}`); err == nil ||
			!strings.Contains(err.Error(), "most queued turns") {
			t.Fatalf("err = %v, want the inbox-full message", err)
		}
		_ = l
	})
}

// --- happy: resume a settled worker, enqueue on a live one ---

// TestSendResumesASettledWorker is the resume path: a worker finishes a first
// turn with a session id, so send revives it. The response says resumed, with
// position 0 (running now, not waiting), and the queue resumed exactly once.
func TestSendResumesASettledWorker(t *testing.T) {
	d, l := sendDeps(t)
	id := delegateOne(t, d, "turn-1")
	waitState(t, d, id, workers.Running)
	l.proc("turn-1").finish("first result", nil)
	waitState(t, d, id, workers.Done)

	data, err := callSend(t, d, `{"job_id":"`+id+`","task":"turn-2"}`)
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if data["state"] != "resumed" {
		t.Errorf("state = %v, want resumed", data["state"])
	}
	if pos, _ := data["position"].(float64); pos != 0 {
		t.Errorf("position = %v, want 0 for a resumed turn", data["position"])
	}
	if data["job"] != id {
		t.Errorf("job = %v, want %s", data["job"], id)
	}
	waitState(t, d, id, workers.Running)
	if l.resumeCount() != 1 {
		t.Errorf("resume count = %d, want 1", l.resumeCount())
	}
}

// TestSendEnqueuesOnALiveWorkerAndItRunsAfter is the enqueue path: a send while
// the worker is on a turn does not resume; it waits in the inbox and runs when
// the live turn settles. The response says queued, with position 1 (first
// waiting turn), and both turns deliver in order through collect.
func TestSendEnqueuesOnALiveWorkerAndItRunsAfter(t *testing.T) {
	d, l := sendDeps(t)
	id := delegateOne(t, d, "turn-1")
	waitState(t, d, id, workers.Running)

	data, err := callSend(t, d, `{"job_id":"`+id+`","task":"turn-2"}`)
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if data["state"] != "queued" {
		t.Errorf("state = %v, want queued", data["state"])
	}
	if pos, _ := data["position"].(float64); pos != 1 {
		t.Errorf("position = %v, want 1 for the first waiting turn", data["position"])
	}
	if l.resumeCount() != 0 {
		t.Fatalf("a live worker was resumed before its turn settled: %d", l.resumeCount())
	}

	// Settle the first turn: the queued turn resumes.
	l.proc("turn-1").finish("first result", nil)
	deadline := time.Now().Add(2 * time.Second)
	for l.resumeCount() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if l.resumeCount() != 1 {
		t.Fatalf("the queued turn did not resume after the live turn settled")
	}
	l.proc("turn-2").finish("second result", nil)

	c := d.Work.Collect(context.Background(), 0, true, 5*time.Second)
	if len(c.Deliveries) != 2 || c.Deliveries[0].Text != "first result" || c.Deliveries[1].Text != "second result" {
		t.Fatalf("deliveries = %+v, want both turns in arrival order", c.Deliveries)
	}
}

// TestSendPositionCountsUpAsTheInboxFills checks the reported position is the
// turn's slot in the inbox: the second waiting turn is position 2.
func TestSendPositionCountsUpAsTheInboxFills(t *testing.T) {
	d, _ := sendDeps(t)
	id := delegateOne(t, d, "turn-1")
	waitState(t, d, id, workers.Running)

	first, _ := callSend(t, d, `{"job_id":"`+id+`","task":"t2"}`)
	if pos, _ := first["position"].(float64); pos != 1 {
		t.Errorf("first waiting turn position = %v, want 1", first["position"])
	}
	second, _ := callSend(t, d, `{"job_id":"`+id+`","task":"t3"}`)
	if pos, _ := second["position"].(float64); pos != 2 {
		t.Errorf("second waiting turn position = %v, want 2", second["position"])
	}
}

// --- concurrency: send does not block collect, cancel or status ---

// TestSendDoesNotBlockCollectCancelStatus is the regra-2 guarantee: while a
// turn runs and a send has queued more, collect, cancel and status all answer
// at once, never serialised behind the live turn.
func TestSendDoesNotBlockCollectCancelStatus(t *testing.T) {
	d, _ := sendDeps(t)
	id := delegateOne(t, d, "turn-1")
	waitState(t, d, id, workers.Running)
	if _, err := callSend(t, d, `{"job_id":"`+id+`","task":"turn-2"}`); err != nil {
		t.Fatalf("send: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		// status answers with the pending turn count (the status tool reads this).
		if views := d.Work.Workers(); len(views) != 1 || views[0].Pending != 1 {
			t.Errorf("status = %+v, want one worker with one pending turn", views)
		}
		// cancel answers and settles the running worker.
		if acted := d.Work.Cancel([]string{id}); len(acted) != 1 {
			t.Errorf("cancel acted on %v, want %s", acted, id)
		}
		// collect answers without blocking.
		_ = d.Work.Collect(context.Background(), 0, false, 0)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("collect/cancel/status blocked behind the live turn + a queued send")
	}
}

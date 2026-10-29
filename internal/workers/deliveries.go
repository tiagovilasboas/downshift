// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"context"
	"time"
	"unicode/utf8"
)

// Delivery is a worker's end, kept until the caller acknowledges it. A delivery
// that was handed out before and not acknowledged comes back with Repeat set.
type Delivery struct {
	Seq     uint64 `json:"seq"`
	Job     string `json:"job"`
	Outcome string `json:"outcome"`
	Text    string `json:"text"`
	Repeat  bool   `json:"repeat"`
	// Tokens is what the worker reported spending; absent when its CLI wrote no
	// usage (a killed or timed-out worker, or a launch that never started).
	Tokens *Tokens `json:"tokens,omitempty"`
}

// Collection is what Collect hands back.
type Collection struct {
	Deliveries []Delivery
	// Ack is the highest sequence returned, 0 when nothing was returned. The
	// caller passes it back to acknowledge.
	Ack uint64
	// Active counts the jobs still queued or running.
	Active int
}

// deliveries keeps the queue's results in order. Sequence numbers start from the
// process start time in milliseconds, so an acknowledgement from before a
// restart cannot match a newer result (spec R20).
type deliveries struct {
	seq     uint64
	pending []entry
}

type entry struct {
	d      Delivery
	handed bool
}

func newDeliveries(start time.Time) deliveries {
	return deliveries{seq: uint64(start.UnixMilli())}
}

func (x *deliveries) add(job, outcome, text string, tokens *Tokens) {
	x.seq++
	x.pending = append(x.pending, entry{d: Delivery{Seq: x.seq, Job: job, Outcome: outcome, Text: capText(text), Tokens: tokens}})
}

// ack drops every delivery with a sequence at or below upTo.
func (x *deliveries) ack(upTo uint64) {
	keep := x.pending[:0]
	for _, e := range x.pending {
		if e.d.Seq > upTo {
			keep = append(keep, e)
		}
	}
	x.pending = keep
}

// take returns every pending delivery and marks each as handed out.
func (x *deliveries) take() []Delivery {
	out := make([]Delivery, 0, len(x.pending))
	for i := range x.pending {
		e := &x.pending[i]
		d := e.d
		d.Repeat = e.handed
		e.handed = true
		out = append(out, d)
	}
	return out
}

// capText keeps the first maxOutput bytes of a report, cut on a character
// boundary, and says so when it cuts.
func capText(s string) string {
	if len(s) <= maxOutput {
		return s
	}
	cut := maxOutput
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "\n[cut: the report is longer than 16000 bytes]"
}

// Collect acknowledges up to ack, optionally waits until nothing is queued or
// running (or the timeout or ctx ends), and returns what is pending.
func (q *Queue) Collect(ctx context.Context, ack uint64, wait bool, timeout time.Duration) Collection {
	q.mu.Lock()
	q.deliv.ack(ack)
	q.mu.Unlock()
	if wait {
		q.waitIdle(ctx, timeout)
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	items := q.deliv.take()
	c := Collection{Deliveries: items, Active: q.active()}
	if len(items) > 0 {
		c.Ack = items[len(items)-1].Seq
	}
	return c
}

// waitIdle returns when no job is active, the timeout ends or ctx is done.
func (q *Queue) waitIdle(ctx context.Context, timeout time.Duration) {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for {
		q.mu.Lock()
		idle := q.active() == 0
		changed := q.changed
		q.mu.Unlock()
		if idle {
			return
		}
		select {
		case <-changed:
		case <-deadline.C:
			return
		case <-ctx.Done():
			return
		}
	}
}

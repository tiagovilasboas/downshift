// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

// GrossSaved is raw - emitted for the run: what the compaction cut, before any
// of it was fetched back. Zero when the run did not shrink.
func (e Event) GrossSaved() int64 {
	if e.RawBytes > e.EmittedBytes {
		return e.RawBytes - e.EmittedBytes
	}
	return 0
}

// NetSaved is the run's saving once the bytes `retrieve` fetched back for it
// are taken off, floored at zero: a run whose output was retrieved in full
// saved nothing. It is the one per-run saving every surface reads.
func (e Event) NetSaved() int64 {
	if net := e.GrossSaved() - e.RetrievedBytes; net > 0 {
		return net
	}
	return 0
}

// attachRetrievals charges each retrieval to the runs that kept its id and sets
// their RetrievedBytes. evs is in log order and is changed in place.
//
// The bytes retrieved for an id are pooled (an id fetched twice counts twice).
// Two runs can share an id when their redacted output is identical; the pool is
// then spent on the oldest run first, up to that run's saving, and the rest on
// the next. The newest run with the id takes whatever is left, so the bytes
// shown as retrieved always add up to what was fetched for ids the log knows.
// A retrieval whose id no run in the log kept (rotated away, another machine)
// charges nothing.
func attachRetrievals(evs []Event, rets []Retrieval) {
	if len(rets) == 0 {
		return
	}
	pool := map[string]int64{}
	for _, r := range rets {
		pool[r.ID] += r.Bytes
	}
	last := map[string]int{}
	for i, ev := range evs {
		if ev.KeptID != "" && pool[ev.KeptID] > 0 {
			last[ev.KeptID] = i
		}
	}
	for i := range evs {
		id := evs[i].KeptID
		if id == "" || pool[id] <= 0 {
			continue
		}
		take := min(pool[id], evs[i].GrossSaved())
		if last[id] == i {
			take = pool[id]
		}
		evs[i].RetrievedBytes = take
		pool[id] -= take
	}
}

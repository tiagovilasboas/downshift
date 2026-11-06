// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package discover

import "sort"

// Stat counts commands for one harness or for all of them.
type Stat struct {
	Harness    string         `json:"harness,omitempty"`
	Commands   int            `json:"commands"`
	Wrapped    int            `json:"wrapped"`
	Bare       int            `json:"bare"`
	BareBytes  int64          `json:"bare_bytes"`
	Skipped    int            `json:"skipped"`
	SkipReason map[string]int `json:"skipped_by_reason"`
	// MetacharKind splits the metachar skips by execwrap.MetacharKind.
	MetacharKind map[string]int `json:"metachar_by_kind"`
}

// Missed is one group of bare, compactible commands: a family plus the first
// word of the command. Counts and bytes only.
type Missed struct {
	Family string `json:"family"`
	Binary string `json:"binary"`
	Count  int    `json:"count"`
	Bytes  int64  `json:"bytes"`
}

// Report is the whole answer. It holds no command text.
type Report struct {
	Since     string   `json:"since"`
	Files     int      `json:"files_scanned"`
	Total     Stat     `json:"total"`
	Harnesses []Stat   `json:"harnesses"`
	Top       []Missed `json:"top_missed"`
}

type missKey struct{ family, binary string }

// tally accumulates one harness while its files stream by.
type tally struct {
	stat Stat
	miss map[missKey]*Missed
}

func newTally(harness string) *tally {
	return &tally{
		stat: Stat{Harness: harness, SkipReason: map[string]int{}, MetacharKind: map[string]int{}},
		miss: map[missKey]*Missed{},
	}
}

// add records one command with the byte size of its result (0 when unknown).
func (t *tally) add(c Class, bytes int64) {
	t.stat.Commands++
	switch c.Kind {
	case KindWrapped:
		t.stat.Wrapped++
	case KindBare:
		t.stat.Bare++
		t.stat.BareBytes += bytes
		k := missKey{c.Family, c.Binary}
		m := t.miss[k]
		if m == nil {
			m = &Missed{Family: c.Family, Binary: c.Binary}
			t.miss[k] = m
		}
		m.Count++
		m.Bytes += bytes
	default:
		t.stat.Skipped++
		t.stat.SkipReason[c.Reason]++
		if c.Meta != "" {
			t.stat.MetacharKind[c.Meta]++
		}
	}
}

// merge folds the tallies into a Report with the top n missed groups.
func merge(since string, files int, tallies []*tally, n int) Report {
	r := Report{Since: since, Files: files, Harnesses: []Stat{}, Top: []Missed{}}
	r.Total.SkipReason = map[string]int{}
	r.Total.MetacharKind = map[string]int{}
	all := map[missKey]*Missed{}
	for _, t := range tallies {
		r.Harnesses = append(r.Harnesses, t.stat)
		s := t.stat
		r.Total.Commands += s.Commands
		r.Total.Wrapped += s.Wrapped
		r.Total.Bare += s.Bare
		r.Total.BareBytes += s.BareBytes
		r.Total.Skipped += s.Skipped
		for k, v := range s.SkipReason {
			r.Total.SkipReason[k] += v
		}
		for k, v := range s.MetacharKind {
			r.Total.MetacharKind[k] += v
		}
		for k, m := range t.miss {
			a := all[k]
			if a == nil {
				a = &Missed{Family: k.family, Binary: k.binary}
				all[k] = a
			}
			a.Count += m.Count
			a.Bytes += m.Bytes
		}
	}
	for _, m := range all {
		r.Top = append(r.Top, *m)
	}
	sort.Slice(r.Top, func(i, j int) bool { return topLess(r.Top[i], r.Top[j]) })
	if n >= 0 && len(r.Top) > n {
		r.Top = r.Top[:n]
	}
	return r
}

// topLess orders by bytes, then count, then name, so output is stable.
func topLess(a, b Missed) bool {
	switch {
	case a.Bytes != b.Bytes:
		return a.Bytes > b.Bytes
	case a.Count != b.Count:
		return a.Count > b.Count
	case a.Family != b.Family:
		return a.Family < b.Family
	}
	return a.Binary < b.Binary
}

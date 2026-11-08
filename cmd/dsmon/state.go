// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

// ── state (polling tail) ──────────────────────────────────────────────────────

const (
	// maxKeptEvents and maxKeptAgents bound memory: the widget only shows the
	// last 24 hours and a handful of rows, so older lines are dropped.
	maxKeptEvents = 5000
	maxKeptAgents = 500
)

type state struct {
	all       []event
	agents    []agentEvent
	size      int64
	modTime   time.Time
	agentSize int64
	lastEvent time.Time
	tick      int
}

// readNewLines reads complete lines from f starting at offset up to EOF and
// returns the offset after the last complete line it consumed. A trailing
// line without a newline is left for the next poll (the writer may be
// mid-append), and a line over telemetry.MaxLineBytes is skipped.
func readNewLines(f *os.File, offset int64, handle func(line []byte)) (int64, error) {
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return offset, err
	}
	r := bufio.NewReaderSize(f, 64*1024)
	for {
		line, err := r.ReadBytes('\n')
		if err != nil {
			// io.EOF: line (if any) is an unterminated partial; do not consume it.
			if err == io.EOF {
				return offset, nil
			}
			return offset, err
		}
		offset += int64(len(line))
		if len(line) > telemetry.MaxLineBytes {
			continue
		}
		handle(line)
	}
}

func (s *state) addEvent(line []byte) bool {
	var e event
	if json.Unmarshal(line, &e) != nil || e.Harness == "" {
		return false
	}
	s.all = append(s.all, e)
	return true
}

// prune keeps only the recent window and at most maxKeptEvents events.
func (s *state) prune() {
	kept := s.all[:0]
	for _, e := range s.all {
		if e.isRecent() {
			kept = append(kept, e)
		}
	}
	if len(kept) > maxKeptEvents {
		kept = append([]event(nil), kept[len(kept)-maxKeptEvents:]...)
	}
	s.all = kept
}

func (s *state) reload(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return
	}
	s.all, s.size, s.modTime = nil, 0, fi.ModTime()
	n, _ := readNewLines(f, 0, func(l []byte) { s.addEvent(l); s.prune1() })
	s.size = n
	if len(s.all) > 0 {
		s.lastEvent = time.Now()
	}
}

// prune1 bounds growth while a large file is read: it only trims when the
// slice has doubled past the cap, so the cost stays amortised.
func (s *state) prune1() {
	if len(s.all) > 2*maxKeptEvents {
		s.prune()
	}
}

func (s *state) pollAgents(path string) {
	fi, err := os.Stat(path)
	if err != nil || fi.Size() == s.agentSize {
		return
	}
	if fi.Size() < s.agentSize {
		// file rotated — reload from scratch
		s.agents = nil
		s.agentSize = 0
	}
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	n, _ := readNewLines(f, s.agentSize, func(l []byte) {
		var ae agentEvent
		// Keep privacy-safe lines: new hook events have an empty task but
		// always carry a tool name. Skip only lines with neither.
		if json.Unmarshal(l, &ae) == nil && (ae.Task != "" || ae.Tool != "") {
			s.agents = append(s.agents, ae)
		}
	})
	s.agentSize = n
	if len(s.agents) > maxKeptAgents {
		s.agents = append([]agentEvent(nil), s.agents[len(s.agents)-maxKeptAgents:]...)
	}
}

func (s *state) recentAgents() []agentEvent {
	var out []agentEvent
	for _, a := range s.agents {
		if a.isRecent() {
			out = append(out, a)
		}
	}
	// last 6
	if len(out) > 6 {
		out = out[len(out)-6:]
	}
	return out
}

func (s *state) poll(path string) {
	fi, err := os.Stat(path)
	if err != nil || (fi.Size() == s.size && fi.ModTime().Equal(s.modTime)) {
		s.tick++
		return
	}
	if fi.Size() < s.size {
		s.reload(path)
		return
	}
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	added := 0
	n, _ := readNewLines(f, s.size, func(l []byte) {
		if s.addEvent(l) {
			added++
		}
	})
	if added > 0 {
		s.lastEvent = time.Now()
		s.prune()
	}
	// Advance by the bytes actually consumed, not the size seen by Stat:
	// lines appended between Stat and read are picked up next poll, never
	// read twice or skipped.
	s.size, s.modTime = n, fi.ModTime()
	s.tick++
}

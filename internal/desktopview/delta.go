// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package desktopview sends a repeat read of the same desktop window as the
// difference from the previous one.
//
// An agent driving a desktop app through agent-desktop reads the UI after
// every action to check what happened (observe, act, verify), and most of
// the tree is the same each time. agent-desktop numbers refs depth-first
// over the same nodes, so an unchanged UI gives the same eN in every
// snapshot; only the snapshot id changes, and refs of an older snapshot stay
// valid (it keeps up to 128). The compact view (compressor/agent_desktop.go)
// writes refs as eN, so a line that is the same text in both views is the
// same element with the same eN, and only the lines that differ need to be
// sent.
package desktopview

import (
	"fmt"
	"strings"
	"time"
)

// MaxDeltaShare is the largest delta, as a share of the full view, that is
// sent instead of the view.
const MaxDeltaShare = 0.6

// maxDiffCells bounds the diff table; a larger changed region is sent whole.
const maxDiffCells = 4_000_000

// view is a compact snapshot view split into its parts.
type view struct {
	header  string   // the first line
	legends []string // refs:, actions: lines
	sid     string   // the snapshot id of the refs: legend
	body    []string // one line per element
}

// parse splits a snapshot view. It reports false for anything else: a find,
// a view without a ref legend, or a partial read (complete=false,
// truncated=true, identity_unproven=true), whose missing nodes are not
// removals.
func parse(text []byte) (view, bool) {
	lines := strings.Split(strings.TrimSuffix(string(text), "\n"), "\n")
	if len(lines) < 3 || !strings.HasPrefix(lines[0], "agent-desktop snapshot ") {
		return view{}, false
	}
	h := lines[0]
	if !strings.Contains(h, " complete=true") || strings.Contains(h, " truncated=true") || strings.Contains(h, " identity_unproven=true") {
		return view{}, false
	}
	v := view{header: h}
	i := 1
	for ; i < len(lines); i++ {
		l := lines[i]
		if sid, ok := strings.CutPrefix(l, "refs: eN means @"); ok {
			v.sid, _, _ = strings.Cut(sid, ":")
		} else if !strings.HasPrefix(l, "actions: ") {
			break
		}
		v.legends = append(v.legends, l)
	}
	v.body = lines[i:]
	return v, v.sid != "" && len(v.body) > 0
}

// Delta returns what to send for cur when base is the view of the previous
// read of the same scope, age ago. It reports false, and cur is sent whole,
// when either is not a complete snapshot view, the legends differ (the eN
// lines would not mean the same), the changed region is too large to diff,
// or the delta would not be much shorter than cur.
func Delta(base, cur []byte, age time.Duration) ([]byte, bool) {
	b, ok1 := parse(base)
	c, ok2 := parse(cur)
	if !ok1 || !ok2 || !sameLegends(b, c) {
		return nil, false
	}
	ops, ok := diffLines(b.body, c.body)
	if !ok {
		return nil, false
	}
	var out strings.Builder
	out.WriteString(c.header + "\n")
	for _, l := range c.legends {
		out.WriteString(l + "\n")
	}
	changed := writeHunks(&out, ops, c.body)
	intro := fmt.Sprintf("delta from snapshot %s (same window, read %s ago): %d of %d lines changed. "+
		"Every other line of that view still holds, and its eN refs are valid in both snapshots. "+
		"If you do not have snapshot %s, read the full output below.\n",
		b.sid, age.Round(time.Second), changed, len(c.body), b.sid)
	if changed == 0 {
		intro = fmt.Sprintf("no change from snapshot %s (same window, read %s ago): every line of that view still holds, "+
			"and its eN refs are valid in both snapshots. If you do not have snapshot %s, read the full output below.\n",
			b.sid, age.Round(time.Second), b.sid)
	}
	text := out.String()
	i := strings.Index(text, "\n") + 1
	for _, l := range c.legends {
		i += len(l) + 1
	}
	text = text[:i] + intro + text[i:]
	if float64(len(text)) > MaxDeltaShare*float64(len(cur)) {
		return nil, false
	}
	return []byte(text), true
}

// sameLegends reports whether the legends of a and b say the same thing
// apart from the snapshot id.
func sameLegends(a, b view) bool {
	if len(a.legends) != len(b.legends) {
		return false
	}
	for i := range a.legends {
		if strings.ReplaceAll(a.legends[i], a.sid, b.sid) != b.legends[i] {
			return false
		}
	}
	return true
}

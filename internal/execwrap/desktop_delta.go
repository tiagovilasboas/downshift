// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"bytes"
	"regexp"
	"strconv"
)

// noFoldStored lists the families whose compact output is never folded by
// line shape once the raw output is stored. An agent-desktop view has one
// line per element, each with the ref the agent acts on; folding same-shape
// lines would hide refs behind a retrieve.
var noFoldStored = map[Family]bool{FamilyAgentDesktop: true}

// reKeptMarker reads the CCR id from MarkerWithID.
var reKeptMarker = regexp.MustCompile(`; full output: downshift retrieve ([0-9a-f]{12})\]\n$`)

// applyDesktop runs the Desktop step on an agent-desktop view whose raw
// output was kept: the delta it returns replaces the view, and the marker is
// rewritten with the new size and the same id. Without a kept raw output
// there is nothing to fall back on, so the view is printed whole. A panic in
// the step prints out unchanged.
func applyDesktop(step func(view []byte) []byte, family Family, raw, out []byte) (res []byte) {
	res = out
	if step == nil || family != FamilyAgentDesktop {
		return out
	}
	defer func() {
		if recover() != nil {
			res = out
		}
	}()
	i := bytes.LastIndex(out, []byte(markerPrefix))
	if i < 0 {
		return out
	}
	m := reKeptMarker.FindSubmatch(out[i:])
	if m == nil {
		return out
	}
	view := append([]byte(nil), out[:i]...)
	next := step(view)
	if len(next) == 0 || bytes.Equal(next, view) {
		return out
	}
	return append(next, MarkerWithID(len(raw), len(next), string(m[1]))...)
}

// DesktopScope is the key a Desktop step stores a view under: the session
// and the exact command line, so only the same read in the same session is
// diffed. It is "" (no delta) without a session.
func DesktopScope(session string, argv []string) string {
	if session == "" {
		return ""
	}
	var b bytes.Buffer
	b.WriteString(session)
	for _, a := range argv {
		b.WriteString("\x00" + strconv.Quote(a))
	}
	return b.String()
}

// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import "strings"

// agentDesktopFindView renders a default find (data.matches): the header, the
// ref legend and one line per match in the node grammar. A match always
// carries value, states and interactive, so their usual values are left out
// and the find legend says which:
//
//	match fields left out: value=null [] interactive=true
//
// A match whose path is the one of the match above is written path=^.
//
// --count, --first, --last and --nth print one small object; they stay raw.
func agentDesktopFindView(env agentDesktopEnvelope) ([]byte, bool) {
	raw, ok := env.Data["matches"]
	if !ok {
		return nil, false
	}
	matches, ok := decodeDesktopValue(raw).([]any)
	if !ok {
		return nil, false
	}
	cx := desktopCtx{sid: desktopSnapshotID(env.Data)}
	var b strings.Builder
	if !writeDesktopHeader(&b, env, "matches") {
		return nil, false
	}
	writeDesktopRefLegend(&b, cx.sid)
	b.WriteString(desktopFindDefaults + "\n")
	prevPath := ""
	for _, m := range matches {
		match, ok := m.(map[string]any)
		if !ok {
			return nil, false
		}
		trimDesktopFindDefaults(match)
		same := false
		if p, ok := match["path"]; ok {
			cur := desktopJSON(p)
			same = cur == prevPath
			prevPath = cur
			if same {
				delete(match, "path")
			}
		}
		var line strings.Builder
		if !writeDesktopNode(&line, match, 0, cx) {
			return nil, false
		}
		if same {
			b.WriteString(strings.TrimSuffix(line.String(), "\n") + " path=^\n")
		} else {
			b.WriteString(line.String())
		}
	}
	return []byte(b.String()), true
}

// desktopFindDefaults is the legend line of a find view.
const desktopFindDefaults = "match fields left out: value=null [] interactive=true; path=^ is the path of the line above"

// trimDesktopFindDefaults removes the fields that hold their usual value.
// The other match fields (ref_id null, path) are written in full.
func trimDesktopFindDefaults(match map[string]any) {
	if v, ok := match["value"]; ok && v == nil {
		delete(match, "value")
	}
	if list, ok := match["states"].([]any); ok && len(list) == 0 {
		delete(match, "states")
	}
	if v, ok := match["interactive"].(bool); ok && v {
		delete(match, "interactive")
	}
}

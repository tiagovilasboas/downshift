// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/lazyre"
)

// The short forms of the node grammar (agent_desktop_node.go). Each one
// writes its field only when the short form carries it whole, and then
// removes it from rest; otherwise it leaves the field for the key=<json>
// fallback.

// desktopStates writes states as [a,b].
func desktopStates(rest map[string]any) string {
	s, ok := joinDesktopWords(rest["states"], reDesktopToken)
	if !ok {
		return ""
	}
	delete(rest, "states")
	return "[" + s + "]"
}

// desktopActions writes available_actions as actions=A,B.
func desktopActions(rest map[string]any) string {
	s, ok := joinDesktopWords(rest["available_actions"], reDesktopWord)
	if !ok {
		return ""
	}
	delete(rest, "available_actions")
	return "actions=" + s
}

// desktopBox writes bounds as box=x,y,w,h when it holds exactly x, y, width
// and height, all numbers.
func desktopBox(rest map[string]any) string {
	m, ok := rest["bounds"].(map[string]any)
	if !ok || len(m) != 4 {
		return ""
	}
	parts := make([]string, 0, 4)
	for _, k := range []string{"x", "y", "width", "height"} {
		n, ok := desktopNumber(m[k])
		if !ok {
			return ""
		}
		parts = append(parts, n)
	}
	delete(rest, "bounds")
	return "box=" + parts[0] + "," + parts[1] + "," + parts[2] + "," + parts[3]
}

// desktopNativeID writes native_id as id=kind:"value".
func desktopNativeID(rest map[string]any) string {
	m, ok := rest["native_id"].(map[string]any)
	if !ok || len(m) != 2 {
		return ""
	}
	kind, ok1 := m["kind"].(string)
	value, ok2 := m["value"].(string)
	if !ok1 || !ok2 || !isDesktopToken(kind) {
		return ""
	}
	delete(rest, "native_id")
	return "id=" + kind + ":" + desktopJSON(value)
}

// desktopMore writes children_count as more=N.
func desktopMore(rest map[string]any) string {
	n, ok := desktopNumber(rest["children_count"])
	if !ok || !reDesktopRefN.MatchString(n) {
		return ""
	}
	delete(rest, "children_count")
	return "more=" + n
}

// desktopCut writes subtree_truncated: true as cut.
func desktopCut(rest map[string]any) string {
	if v, ok := rest["subtree_truncated"].(bool); !ok || !v {
		return ""
	}
	delete(rest, "subtree_truncated")
	return "cut"
}

// reDesktopClass is a CSS class name that needs no quoting after a dot.
var reDesktopClass = lazyre.New(`^[A-Za-z_-][A-Za-z0-9_-]*$`)

// desktopClasses writes dom_classes as .a.b.
func desktopClasses(rest map[string]any) string {
	s, ok := joinDesktopWords(rest["dom_classes"], reDesktopClass)
	if !ok {
		return ""
	}
	delete(rest, "dom_classes")
	return "." + strings.ReplaceAll(s, ",", ".")
}

// desktopDefaultActions handles available_actions on an eN node against the
// legend's default list: the default is left out, a node with no actions is
// written actions=-, and any other list is left for desktopActions.
func desktopDefaultActions(rest map[string]any, def string) string {
	v, ok := rest["available_actions"]
	if !ok {
		return "actions=-"
	}
	if s, ok := joinDesktopWords(v, reDesktopWord); ok && s == def {
		delete(rest, "available_actions")
	}
	return ""
}

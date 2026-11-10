// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/lazyre"
)

// The node line grammar. One node per line, two spaces of indent per depth:
//
//	[eN | ref=<json>] <role> ["<name>"] [<field>]... [<key>=<json>]...
//
//	eN           ref_id "@<snapshot_id>:eN" of this snapshot (see the legend)
//	ref=<json>   any other ref_id, verbatim
//	"<name>"     name, a JSON string right after the role ("" is kept)
//	[a,b]        states, when every state is a plain word
//	actions=A,B  available_actions, when every action is a plain word. On an
//	             eN node the legend's default list is left out, and
//	             actions=- says the node has none

//	box=x,y,w,h  bounds, when it holds exactly those four numbers
//	id=kind:"v"  native_id, when kind is a plain word and value a string
//	.a.b         dom_classes, when every class is a plain word
//	more=N       children_count: N children not read (depth limit)
//	cut          subtree_truncated: true
//	value=, desc=, ph=, rdesc=, sub=, hint=, classes=
//	             value, description, placeholder, role_description,
//	             subrole, hint, dom_classes (when not .a.b), as JSON
//	key=<json>   any other field, verbatim, sorted by key
//
// A field that does not fit its short form is written as key=<json> under
// its own JSON name, so nothing is ever dropped.

var (
	reDesktopToken = lazyre.New(`^[a-z][a-z0-9_-]*$`)
	reDesktopWord  = lazyre.New(`^[A-Za-z][A-Za-z0-9_-]*$`)
	reDesktopID    = lazyre.New(`^[A-Za-z0-9_-]{3,64}$`)
	reDesktopRefN  = lazyre.New(`^[1-9][0-9]{0,9}$`)
)

func isDesktopToken(s string) bool { return reDesktopToken.MatchString(s) }
func isDesktopID(s string) bool    { return reDesktopID.MatchString(s) }

// desktopJSONFields are written as short-key=<json> (short key, JSON name).
var desktopJSONFields = [][2]string{
	{"value", "value"}, {"desc", "description"}, {"ph", "placeholder"},
	{"rdesc", "role_description"}, {"sub", "subrole"}, {"hint", "hint"},
	{"classes", "dom_classes"},
}

// desktopCtx is what every line of one view shares: the snapshot id of the
// short refs, and the action list a short-ref node has unless its line says
// otherwise (see desktopDefaultActions).
type desktopCtx struct {
	sid, actions string
}

// writeDesktopNode writes node and its children. It reports false when the
// node has a shape the grammar cannot carry (no role, a role that is not a
// plain word, children that are not a list of objects).
func writeDesktopNode(b *strings.Builder, node map[string]any, depth int, cx desktopCtx) bool {
	role, ok := node["role"].(string)
	if !ok || !isDesktopToken(role) {
		return false
	}
	rest := make(map[string]any, len(node))
	for k, v := range node {
		rest[k] = v
	}
	delete(rest, "role")
	children, hasKids := rest["children"]
	delete(rest, "children")
	b.WriteString(strings.Repeat("  ", depth))
	if !writeDesktopFields(b, role, rest, cx) {
		return false
	}
	b.WriteByte('\n')
	if !hasKids {
		return true
	}
	list, ok := children.([]any)
	if !ok {
		return false
	}
	for _, c := range list {
		child, ok := c.(map[string]any)
		if !ok || !writeDesktopNode(b, child, depth+1, cx) {
			return false
		}
	}
	return true
}

// writeDesktopFields writes the line of one node (without indent or newline),
// consuming the fields it writes from rest.
func writeDesktopFields(b *strings.Builder, role string, rest map[string]any, cx desktopCtx) bool {
	short := false
	if ref, ok := rest["ref_id"]; ok {
		r := desktopRef(ref, cx.sid)
		short = !strings.HasPrefix(r, "ref=")
		b.WriteString(r + " ")
		delete(rest, "ref_id")
	}
	b.WriteString(role)
	if name, ok := rest["name"].(string); ok {
		b.WriteString(" " + desktopJSON(name))
		delete(rest, "name")
	}
	if short && cx.actions != "" {
		if s := desktopDefaultActions(rest, cx.actions); s != "" {
			b.WriteString(" " + s)
		}
	}
	for _, form := range []func(map[string]any) string{desktopStates, desktopActions, desktopBox, desktopNativeID, desktopClasses, desktopMore, desktopCut} {
		if s := form(rest); s != "" {
			b.WriteString(" " + s)
		}
	}
	for _, f := range desktopJSONFields {
		if v, ok := rest[f[1]]; ok {
			b.WriteString(" " + f[0] + "=" + desktopJSON(v))
			delete(rest, f[1])
		}
	}
	keys := make([]string, 0, len(rest))
	for k := range rest {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !isDesktopToken(k) || desktopReservedKey[k] {
			return false
		}
		b.WriteString(" " + k + "=" + desktopJSON(rest[k]))
	}
	return true
}

// desktopReservedKey names the words a verbatim key=<json> field must not
// use, because the grammar gives them another meaning.
var desktopReservedKey = map[string]bool{
	"ref": true, "actions": true, "box": true, "id": true, "more": true, "cut": true,
	"desc": true, "ph": true, "rdesc": true, "sub": true, "classes": true, "value": true, "hint": true,
}

// desktopRef is eN for a ref of this snapshot, else ref=<json>.
func desktopRef(ref any, sid string) string {
	if s, ok := ref.(string); ok && sid != "" {
		if n, found := strings.CutPrefix(s, "@"+sid+":e"); found && reDesktopRefN.MatchString(n) {
			return "e" + n
		}
	}
	return "ref=" + desktopJSON(ref)
}

// joinDesktopWords joins a list of plain words with commas, or reports false.
func joinDesktopWords(v any, word *lazyre.Regexp) (string, bool) {
	list, ok := v.([]any)
	if !ok || len(list) == 0 {
		return "", false
	}
	words := make([]string, len(list))
	for i, item := range list {
		s, ok := item.(string)
		if !ok || !word.MatchString(s) {
			return "", false
		}
		words[i] = s
	}
	return strings.Join(words, ","), true
}

// desktopNumber reports whether v is a JSON number (decoded with UseNumber).
func desktopNumber(v any) (string, bool) {
	n, ok := v.(json.Number)
	return string(n), ok
}

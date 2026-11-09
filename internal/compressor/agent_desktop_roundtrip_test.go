// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// The agent-desktop view claims to keep every field. This decoder reads a
// view back into the envelope by the grammar in agent_desktop_node.go, and
// the tests compare it with the input as parsed JSON (key order is not a
// fact: the CLI sorts data keys, MCP sorts every key).

var (
	reViewHeader  = regexp.MustCompile(`^agent-desktop ([a-z][a-z0-9_-]*) \(envelope ([^)]+)\)`)
	reViewRefs    = regexp.MustCompile(`^refs: eN means @([A-Za-z0-9_-]+):eN$`)
	reViewActions = regexp.MustCompile(`^actions: an eN line without actions= has (\S+); actions=- means none$`)
	reViewShortN  = regexp.MustCompile(`^e[1-9][0-9]*$`)
)

type viewDecoder struct {
	sid, actions string
	find         bool
	prevPath     any
}

func decodeDesktopView(t *testing.T, view string) map[string]any {
	t.Helper()
	lines := strings.Split(strings.TrimSuffix(view, "\n"), "\n")
	m := reViewHeader.FindStringSubmatch(lines[0])
	if m == nil {
		t.Fatalf("no header: %q", lines[0])
	}
	data := map[string]any{}
	rest := lines[0][len(m[0]):]
	for rest != "" {
		key, val, tail := splitKeyJSON(t, strings.TrimPrefix(rest, " "))
		data[key], rest = val, tail
	}
	d := &viewDecoder{find: m[1] == "find"}
	lines = lines[1:]
	for len(lines) > 0 {
		if s := reViewRefs.FindStringSubmatch(lines[0]); s != nil {
			d.sid = s[1]
		} else if s := reViewActions.FindStringSubmatch(lines[0]); s != nil {
			d.actions = s[1]
		} else if !strings.HasPrefix(lines[0], "match fields left out:") {
			break
		}
		lines = lines[1:]
	}
	if d.find {
		var matches []any
		for _, l := range lines {
			matches = append(matches, d.node(t, l))
		}
		if matches == nil {
			matches = []any{}
		}
		data["matches"] = matches
	} else {
		data["tree"] = d.tree(t, lines)
	}
	return map[string]any{"version": m[2], "ok": true, "command": m[1], "data": data}
}

// tree rebuilds nested children from the indent of each line.
func (d *viewDecoder) tree(t *testing.T, lines []string) map[string]any {
	var stack []map[string]any
	var root map[string]any
	for _, l := range lines {
		trimmed := strings.TrimLeft(l, " ")
		depth := (len(l) - len(trimmed)) / 2
		n := d.node(t, trimmed)
		if depth == 0 {
			root, stack = n, []map[string]any{n}
			continue
		}
		parent := stack[depth-1]
		kids, _ := parent["children"].([]any)
		parent["children"] = append(kids, n)
		stack = append(stack[:depth], n)
	}
	return root
}

// node decodes one line.
func (d *viewDecoder) node(t *testing.T, line string) map[string]any {
	t.Helper()
	n := map[string]any{}
	word, rest, _ := strings.Cut(line, " ")
	short := false
	switch {
	case reViewShortN.MatchString(word):
		n["ref_id"], short = "@"+d.sid+":"+word, true
		word, rest, _ = strings.Cut(rest, " ")
	case strings.HasPrefix(word, "ref="):
		_, v, tail := splitKeyJSON(t, line)
		n["ref_id"] = v
		word, rest, _ = strings.Cut(strings.TrimPrefix(tail, " "), " ")
	}
	n["role"] = word
	if strings.HasPrefix(rest, `"`) {
		v, tail := readJSON(t, rest)
		n["name"], rest = v, strings.TrimPrefix(tail, " ")
	}
	for rest != "" {
		rest = d.field(t, n, rest)
	}
	if short && d.actions != "" {
		if _, ok := n["available_actions"]; !ok {
			n["available_actions"] = words(d.actions)
		} else if n["available_actions"] == nil {
			delete(n, "available_actions")
		}
	}
	if d.find {
		d.findDefaults(n)
	}
	return n
}

// field decodes one field at the start of rest and returns what follows it.
func (d *viewDecoder) field(t *testing.T, n map[string]any, rest string) string {
	t.Helper()
	switch {
	case strings.HasPrefix(rest, "["):
		end := strings.Index(rest, "]")
		n["states"] = words(rest[1:end])
		return strings.TrimPrefix(rest[end+1:], " ")
	case strings.HasPrefix(rest, "."):
		w, tail, _ := strings.Cut(rest, " ")
		n["dom_classes"] = words(strings.ReplaceAll(w[1:], ".", ","))
		return tail
	case rest == "cut" || strings.HasPrefix(rest, "cut "):
		n["subtree_truncated"] = true
		return strings.TrimPrefix(rest[3:], " ")
	}
	key, val, _ := strings.Cut(rest, "=")
	plain := map[string]bool{"actions": true, "box": true, "id": true, "more": true}
	if plain[key] || (key == "path" && strings.HasPrefix(val, "^")) {
		w, tail, _ := strings.Cut(val, " ")
		d.plainField(t, n, key, w)
		return tail
	}
	_, v, tail := splitKeyJSON(t, rest)
	long := map[string]string{"desc": "description", "ph": "placeholder", "rdesc": "role_description", "sub": "subrole", "classes": "dom_classes"}
	if l, ok := long[key]; ok {
		key = l
	}
	n[key] = v
	if key == "path" {
		d.prevPath = v
	}
	return strings.TrimPrefix(tail, " ")
}

func (d *viewDecoder) plainField(t *testing.T, n map[string]any, key, w string) {
	switch key {
	case "actions":
		if w == "-" {
			n["available_actions"] = nil
		} else {
			n["available_actions"] = words(w)
		}
	case "box":
		p := strings.Split(w, ",")
		n["bounds"] = map[string]any{"x": json.Number(p[0]), "y": json.Number(p[1]), "width": json.Number(p[2]), "height": json.Number(p[3])}
	case "id":
		kind, v, _ := strings.Cut(w, ":")
		val, _ := readJSON(t, v)
		n["native_id"] = map[string]any{"kind": kind, "value": val}
	case "more":
		n["children_count"] = json.Number(w)
	case "path":
		n["path"] = d.prevPath
	}
}

func (d *viewDecoder) findDefaults(n map[string]any) {
	if _, ok := n["value"]; !ok {
		n["value"] = nil
	}
	if _, ok := n["states"]; !ok {
		n["states"] = []any{}
	}
	if _, ok := n["interactive"]; !ok {
		n["interactive"] = true
	}
}

func words(s string) []any {
	var out []any
	for _, w := range strings.Split(s, ",") {
		out = append(out, w)
	}
	return out
}

// splitKeyJSON reads key=<json> at the start of s.
func splitKeyJSON(t *testing.T, s string) (string, any, string) {
	t.Helper()
	key, val, ok := strings.Cut(s, "=")
	if !ok {
		t.Fatalf("no key=value in %q", s)
	}
	v, tail := readJSON(t, val)
	return key, v, tail
}

// readJSON reads one JSON value at the start of s.
func readJSON(t *testing.T, s string) (any, string) {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		t.Fatalf("bad JSON at %q: %v", s, err)
	}
	return v, s[dec.InputOffset():]
}

func parseEnvelope(t *testing.T, in []byte) map[string]any {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(in))
	dec.UseNumber()
	var v map[string]any
	if err := dec.Decode(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

func assertRoundTrip(t *testing.T, in []byte) Result {
	t.Helper()
	r := CompressCommandOutput(in, FormatAgentDesktop, 0)
	if !r.Applied {
		t.Fatalf("not applied: %s", r.Reason)
	}
	got, want := decodeDesktopView(t, string(r.Output)), parseEnvelope(t, in)
	if !reflect.DeepEqual(got, want) {
		gj, _ := json.Marshal(got)
		wj, _ := json.Marshal(want)
		t.Fatalf("view does not decode to the input\nview:\n%s\ngot:  %s\nwant: %s", r.Output, gj, wj)
	}
	return r
}

func TestAgentDesktopViewKeepsEveryFieldOfTheCorpus(t *testing.T) {
	for _, name := range []string{"agent_desktop_snapshot_files.txt", "agent_desktop_snapshot_chat.txt", "agent_desktop_find_buttons.txt"} {
		t.Run(name, func(t *testing.T) {
			r := assertRoundTrip(t, []byte(readCorpus(t, name)))
			t.Log(fmt.Sprintf("%d -> %d bytes", r.OriginalBytes, r.ReducedBytes))
		})
	}
}

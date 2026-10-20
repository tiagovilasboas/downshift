// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

// ---- merge ---------------------------------------------------------------

// quoteCommand renders `<binary> <args>`, quoting the binary when needed.
func quoteCommand(binary, args string) string {
	b := binary
	if strings.ContainsAny(b, " \t") {
		b = `"` + b + `"`
	}
	return b + " " + args
}

// isDownshiftCommand reports whether command runs `downshift <args>` through
// any binary path, so a stale path can be replaced.
func isDownshiftCommand(command, args string) bool {
	c := strings.TrimSpace(command)
	var bin, rest string
	if strings.HasPrefix(c, `"`) {
		end := strings.Index(c[1:], `"`)
		if end < 0 {
			return false
		}
		bin, rest = c[1:1+end], c[2+end:]
	} else if i := strings.IndexAny(c, " \t"); i >= 0 {
		bin, rest = c[:i], c[i:]
	} else {
		bin = c
	}
	base := strings.TrimSuffix(filepath.Base(strings.ReplaceAll(bin, `\`, "/")), ".exe")
	return base == "downshift" && strings.Join(strings.Fields(rest), " ") == strings.Join(strings.Fields(args), " ")
}

// defaultKeySet reports whether the document already holds d's key.
func defaultKeySet(doc *jobj, d setupDefault) bool {
	holder := doc
	if d.Parent != "" {
		pv, ok := doc.get(d.Parent)
		if !ok {
			return false
		}
		holder, ok = asObj(pv)
		if !ok {
			return false
		}
	}
	_, ok := holder.get(d.Key)
	return ok
}

func asObj(v any) (*jobj, bool) { o, ok := v.(*jobj); return o, ok }

// mergeTarget applies every hook (and the status line) of t to doc in place.
// skipOptional leaves out the optional defaults (the main-session policy).
// optIn turns on the defaults that cost money; without it they are only
// suggested, and only when their key is unset.
func mergeTarget(doc *jobj, t setupTarget, binary, stateDir string, skipOptional, optIn bool) ([]string, error) {
	var notes []string
	for _, d := range t.Defaults {
		if skipOptional && d.Optional {
			continue
		}
		if d.OptIn && !optIn {
			if d.Suggest != "" && !defaultKeySet(doc, d) {
				notes = append(notes, d.Suggest)
			}
			continue
		}
		parent := doc
		if d.Parent != "" {
			pv, ok := doc.get(d.Parent)
			if !ok || pv == nil {
				next := newJobj()
				doc.set(d.Parent, next)
				parent = next
			} else if next, isObj := asObj(pv); isObj {
				parent = next
			} else {
				return nil, fmt.Errorf("%q is not a JSON object, left untouched", d.Parent)
			}
		}
		v, ok := parent.get(d.Key)
		if !ok {
			parent.set(d.Key, d.Value)
			continue
		}
		if d.Note != "" && fmt.Sprint(v) != fmt.Sprint(d.Value) {
			notes = append(notes, d.Note)
		}
	}
	if len(t.Hooks) == 0 {
		return mergeStatusLine(doc, t, binary, notes)
	}
	holder := doc
	for _, key := range t.HooksPath {
		v, ok := holder.get(key)
		if !ok || v == nil {
			next := newJobj()
			holder.set(key, next)
			holder = next
			continue
		}
		next, isObj := asObj(v)
		if !isObj {
			return nil, fmt.Errorf("%q is not a JSON object, left untouched", key)
		}
		holder = next
	}
	for _, h := range t.Hooks {
		var list []any
		if v, ok := holder.get(h.Event); ok && v != nil {
			arr, isArr := v.([]any)
			if !isArr {
				return nil, fmt.Errorf("%q is not a JSON array, left untouched", h.Event)
			}
			list = arr
		}
		cmd := quoteCommand(binary, h.Args)
		if h.Wrapper != "" {
			cmd = wrapperPath(stateDir, h)
		}
		if repointElsewhere(holder, h, cmd) {
			continue
		}
		if t.Nested {
			list = ensureNestedHook(list, h, cmd, ownedMatchers(t.Hooks, h.Event))
		} else {
			list = ensureFlatHook(list, h, cmd)
		}
		holder.set(h.Event, list)
	}
	return mergeStatusLine(doc, t, binary, notes)
}

// mergeStatusLine applies the optional statusline default and returns the notes.
func mergeStatusLine(doc *jobj, t setupTarget, binary string, notes []string) ([]string, error) {
	if sl := t.StatusLine; sl != nil {
		note, err := ensureStatusLine(doc, *sl, binary)
		if err != nil {
			return nil, err
		}
		if note != "" {
			notes = append(notes, note)
		}
	}
	return notes, nil
}

// repointElsewhere finds a Downshift hook for h outside holder's own h.Event
// list, such as inside an Antigravity named group, and points it at cmd. It
// reports whether one was found, so setup does not add a duplicate.
func repointElsewhere(holder *jobj, h setupHook, cmd string) bool {
	found := false
	var walk func(v any, under bool)
	walk = func(v any, under bool) {
		switch x := v.(type) {
		case *jobj:
			if c, ok := x.vals["command"].(string); ok && under && ownsCommand(c, h) {
				if c != cmd {
					x.set("command", cmd)
				}
				found = true
			}
			for k, child := range x.vals {
				walk(child, under || k == h.Event)
			}
		case []any:
			for _, item := range x {
				walk(item, under)
			}
		}
	}
	for k, child := range holder.vals {
		if k != h.Event {
			walk(child, false)
		}
	}
	return found
}

func hookCommand(o *jobj) string {
	c, _ := o.vals["command"].(string)
	return c
}

// ownedMatchers lists the non-empty matchers this setup writes for event.
// A group under one of them is another hook of ours, never a stale one to move.
func ownedMatchers(hooks []setupHook, event string) []string {
	var out []string
	for _, h := range hooks {
		if h.Event == event && h.Matcher != "" {
			out = append(out, h.Matcher)
		}
	}
	return out
}

func ensureNestedHook(list []any, h setupHook, cmd string, owned []string) []any {
	for _, item := range list {
		group, ok := asObj(item)
		if !ok {
			continue
		}
		inner, _ := group.vals["hooks"].([]any)
		for j, hk := range inner {
			ho, ok := asObj(hk)
			if !ok || !ownsCommand(hookCommand(ho), h) {
				continue
			}
			cur, _ := group.vals["matcher"].(string)
			if h.Matcher != "" && cur != h.Matcher && slices.Contains(owned, cur) {
				// Another of our hooks (say Bash beside Agent|Task): not ours to move.
				break
			}
			if hookCommand(ho) != cmd {
				ho.set("command", cmd)
			}
			if h.Matcher == "" || cur == h.Matcher {
				return list
			}
			if len(inner) == 1 {
				group.set("matcher", h.Matcher)
				return list
			}
			// The group also holds someone else's hook: leave it and move ours.
			rest := append(append([]any{}, inner[:j]...), inner[j+1:]...)
			group.set("hooks", rest)
			return append(list, newNestedGroup(h, cmd))
		}
	}
	return append(list, newNestedGroup(h, cmd))
}

func newNestedGroup(h setupHook, cmd string) *jobj {
	hk := newJobj()
	hk.set("type", "command")
	hk.set("command", cmd)
	g := newJobj()
	if h.Matcher != "" {
		g.set("matcher", h.Matcher)
	}
	g.set("hooks", []any{hk})
	return g
}

func ensureFlatHook(list []any, h setupHook, cmd string) []any {
	for _, item := range list {
		e, ok := asObj(item)
		if !ok || !ownsCommand(hookCommand(e), h) {
			continue
		}
		if hookCommand(e) != cmd {
			e.set("command", cmd)
		}
		if cur, _ := e.vals["matcher"].(string); h.Matcher != "" && cur != h.Matcher {
			e.set("matcher", h.Matcher)
		}
		return list
	}
	e := newJobj()
	if h.Matcher != "" {
		e.set("matcher", h.Matcher)
	}
	e.set("command", cmd)
	return append(list, e)
}

// ensureStatusLine sets the status line only when none exists. A Downshift
// status line pointing at another binary is repointed; anyone else's is left
// alone and the returned note says how to chain it.
func ensureStatusLine(doc *jobj, sl setupStatusLine, binary string) (string, error) {
	cmd := quoteCommand(binary, sl.Args)
	v, ok := doc.get(sl.Key)
	cur, isObj := asObj(v)
	empty := !ok || v == nil || (isObj && len(cur.keys) == 0) || (isObj && hookCommand(cur) == "" && cur.vals["type"] == nil)
	if empty {
		o := newJobj()
		o.set("type", "command")
		o.set("command", cmd)
		doc.set(sl.Key, o)
		return "", nil
	}
	if isObj && isDownshiftCommand(hookCommand(cur), sl.Args) {
		if hookCommand(cur) != cmd {
			cur.set("command", cmd)
		}
		return "", nil
	}
	existing := ""
	if isObj {
		existing = hookCommand(cur)
	}
	return fmt.Sprintf("%s already set (%s): left untouched. To keep Downshift's quota line, pipe its input to `%s` before rendering (docs/guides/quota.md).",
		sl.Key, existing, quoteCommand(binary, sl.ChainArgs)), nil
}

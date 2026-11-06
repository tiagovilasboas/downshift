// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"fmt"
	"slices"

	"github.com/tiagovilasboas/downshift/internal/setup/configfile"
)

// ownedMatchers lists the non-empty matchers this setup writes for event.
// A group under one of them is another hook of ours, never a stale one to move.
func ownedMatchers(hooks []Hook, event string) []string {
	var out []string
	for _, h := range hooks {
		if h.Event == event && h.Matcher != "" {
			out = append(out, h.Matcher)
		}
	}
	return out
}

func ensureNestedHook(list []any, h Hook, cmd string, owned []string) []any {
	for _, item := range list {
		group, ok := configfile.AsObject(item)
		if !ok {
			continue
		}
		inner, _ := group.Value("hooks").([]any)
		for j, hk := range inner {
			ho, ok := configfile.AsObject(hk)
			if !ok || !ownsCommand(HookCommand(ho), h) {
				continue
			}
			cur, _ := group.Value("matcher").(string)
			if h.Matcher != "" && cur != h.Matcher && slices.Contains(owned, cur) {
				// Another of our hooks (say Bash beside Agent|Task): not ours to move.
				break
			}
			if HookCommand(ho) != cmd {
				ho.Set("command", cmd)
			}
			if h.Matcher == "" || cur == h.Matcher {
				return list
			}
			if len(inner) == 1 {
				group.Set("matcher", h.Matcher)
				return list
			}
			// The group also holds someone else's hook: leave it and move ours.
			rest := append(append([]any{}, inner[:j]...), inner[j+1:]...)
			group.Set("hooks", rest)
			return append(list, newNestedGroup(h, cmd))
		}
	}
	return append(list, newNestedGroup(h, cmd))
}

func newNestedGroup(h Hook, cmd string) *configfile.Object {
	hk := configfile.NewObject()
	hk.Set("type", "command")
	hk.Set("command", cmd)
	g := configfile.NewObject()
	if h.Matcher != "" {
		g.Set("matcher", h.Matcher)
	}
	g.Set("hooks", []any{hk})
	return g
}

func ensureFlatHook(list []any, h Hook, cmd string, owned []string) []any {
	for _, item := range list {
		e, ok := configfile.AsObject(item)
		if !ok || !ownsCommand(HookCommand(e), h) {
			continue
		}
		if cur, _ := e.Value("matcher").(string); h.Matcher != "" && cur != h.Matcher && slices.Contains(owned, cur) {
			// Another of our hooks (say Shell beside Task): not ours to move.
			continue
		}
		if HookCommand(e) != cmd {
			e.Set("command", cmd)
		}
		if cur, _ := e.Value("matcher").(string); h.Matcher != "" && cur != h.Matcher {
			e.Set("matcher", h.Matcher)
		}
		return list
	}
	e := configfile.NewObject()
	if h.Matcher != "" {
		e.Set("matcher", h.Matcher)
	}
	e.Set("command", cmd)
	return append(list, e)
}

// ensureStatusLine sets the status line only when none exists. A Downshift
// status line pointing at another binary is repointed; anyone else's is left
// alone and the returned note says how to chain it.
func ensureStatusLine(doc *configfile.Object, sl StatusLine, binary string) (string, error) {
	cmd := QuoteCommand(binary, sl.Args)
	v, ok := doc.Get(sl.Key)
	cur, isObj := configfile.AsObject(v)
	empty := !ok || v == nil || (isObj && cur.Len() == 0) || (isObj && HookCommand(cur) == "" && cur.Value("type") == nil)
	if empty {
		o := configfile.NewObject()
		o.Set("type", "command")
		o.Set("command", cmd)
		doc.Set(sl.Key, o)
		return "", nil
	}
	if isObj && IsDownshiftCommand(HookCommand(cur), sl.Args) {
		if HookCommand(cur) != cmd {
			cur.Set("command", cmd)
		}
		return "", nil
	}
	existing := ""
	if isObj {
		existing = HookCommand(cur)
	}
	return fmt.Sprintf("%s already set (%s): left untouched. To keep Downshift's quota line, pipe its input to `%s` before rendering (docs/guides/quota.md).",
		sl.Key, existing, QuoteCommand(binary, sl.ChainArgs)), nil
}

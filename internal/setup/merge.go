// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/setup/configfile"
)

// ---- merge ---------------------------------------------------------------

// QuoteCommand renders `<binary> <args>`, quoting the binary when needed.
func QuoteCommand(binary, args string) string {
	b := binary
	if strings.ContainsAny(b, " \t") {
		b = `"` + b + `"`
	}
	return b + " " + args
}

// IsDownshiftCommand reports whether command runs `downshift <args>` through
// any binary path, so a stale path can be replaced.
func IsDownshiftCommand(command, args string) bool {
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
func defaultKeySet(doc *configfile.Object, d Default) bool {
	holder := doc
	if d.Parent != "" {
		pv, ok := doc.Get(d.Parent)
		if !ok {
			return false
		}
		holder, ok = configfile.AsObject(pv)
		if !ok {
			return false
		}
	}
	_, ok := holder.Get(d.Key)
	return ok
}

// MergeTarget applies every hook (and the status line) of t to doc in place.
// skipOptional leaves out the optional defaults (the main-session policy).
// optIn turns on the defaults that cost money; without it they are only
// suggested, and only when their key is unset.
func MergeTarget(doc *configfile.Object, t Target, binary, stateDir string, skipOptional, optIn bool) ([]string, error) {
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
			pv, ok := doc.Get(d.Parent)
			if !ok || pv == nil {
				next := configfile.NewObject()
				doc.Set(d.Parent, next)
				parent = next
			} else if next, isObj := configfile.AsObject(pv); isObj {
				parent = next
			} else {
				return nil, fmt.Errorf("%q is not a JSON object, left untouched", d.Parent)
			}
		}
		v, ok := parent.Get(d.Key)
		if !ok {
			parent.Set(d.Key, d.Value)
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
		v, ok := holder.Get(key)
		if !ok || v == nil {
			next := configfile.NewObject()
			holder.Set(key, next)
			holder = next
			continue
		}
		next, isObj := configfile.AsObject(v)
		if !isObj {
			return nil, fmt.Errorf("%q is not a JSON object, left untouched", key)
		}
		holder = next
	}
	for _, h := range t.Hooks {
		var list []any
		if v, ok := holder.Get(h.Event); ok && v != nil {
			arr, isArr := v.([]any)
			if !isArr {
				return nil, fmt.Errorf("%q is not a JSON array, left untouched", h.Event)
			}
			list = arr
		}
		cmd := QuoteCommand(binary, h.Args)
		if h.Wrapper != "" {
			cmd = WrapperPath(stateDir, h)
		}
		if repointElsewhere(holder, h, cmd) {
			continue
		}
		if t.Nested {
			list = ensureNestedHook(list, h, cmd, ownedMatchers(t.Hooks, h.Event))
		} else {
			list = ensureFlatHook(list, h, cmd, ownedMatchers(t.Hooks, h.Event))
		}
		holder.Set(h.Event, list)
	}
	return mergeStatusLine(doc, t, binary, notes)
}

// mergeStatusLine applies the optional statusline default and returns the notes.
func mergeStatusLine(doc *configfile.Object, t Target, binary string, notes []string) ([]string, error) {
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
func repointElsewhere(holder *configfile.Object, h Hook, cmd string) bool {
	found := false
	var walk func(v any, under bool)
	walk = func(v any, under bool) {
		switch x := v.(type) {
		case *configfile.Object:
			if c, ok := x.Value("command").(string); ok && under && ownsCommand(c, h) {
				if c != cmd {
					x.Set("command", cmd)
				}
				found = true
			}
			for _, k := range x.Keys() {
				walk(x.Value(k), under || k == h.Event)
			}
		case []any:
			for _, item := range x {
				walk(item, under)
			}
		}
	}
	for _, k := range holder.Keys() {
		if k != h.Event {
			walk(holder.Value(k), false)
		}
	}
	return found
}

func HookCommand(o *configfile.Object) string {
	c, _ := o.Value("command").(string)
	return c
}

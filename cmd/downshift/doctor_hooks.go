// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/discovery"
	"github.com/tiagovilasboas/downshift/internal/quota"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// Per-harness hook checks for `doctor`. Harness facts are data: the expected
// hooks come from setupTargets (the table `setup` writes from), plus
// doctorManualWiring for a harness setup cannot write. Nothing here names a
// harness.

// doctorHook is one expected hook.
type doctorHook struct {
	Event   string
	Matcher string
	Args    string // subcommand and arguments after the binary
	Wrapper string // basename of a script that carries the subcommand
	// Required: routing does not work without it. The first hook of a harness
	// is the routing hook; the rest only observe (honor, usage).
	Required bool
}

type doctorWiring struct {
	File   string // relative to HOME
	Nested bool   // entry shape, see setupTarget.Nested
	Setup  bool   // `downshift setup <harness>` can write the file
	Hooks  []doctorHook
}

// doctorManualWiring covers harnesses whose setupTarget has no File because
// setup does not write it. KiroCrew takes only an absolute executable path, so
// its hook is a wrapper script.
var doctorManualWiring = map[string]doctorWiring{
	"kirocrew": {
		File: ".kiro/crew/config.json",
		Hooks: []doctorHook{
			{Event: "preToolUse", Matcher: "*", Args: "kirocrew", Wrapper: "kirocrew-pre-tool-use.sh", Required: true},
		},
	},
}

func doctorWiringFor(harness string) (doctorWiring, bool) {
	for _, t := range setupTargets {
		if t.Name != harness || t.File == "" {
			continue
		}
		w := doctorWiring{File: t.File, Nested: t.Nested, Setup: true}
		for i, h := range t.Hooks {
			w.Hooks = append(w.Hooks, doctorHook{Event: h.Event, Matcher: h.Matcher, Args: h.Args, Required: i == 0})
		}
		return w, true
	}
	w, ok := doctorManualWiring[harness]
	return w, ok
}

// foundCommand is a "command" string in a hook file and the keys above it.
type foundCommand struct {
	Path    []string
	Command string
}

func collectCommands(v any, path []string, out *[]foundCommand) {
	switch t := v.(type) {
	case map[string]any:
		for k, child := range t {
			if s, ok := child.(string); ok && k == "command" {
				*out = append(*out, foundCommand{Path: path, Command: s})
				continue
			}
			collectCommands(child, append(append([]string(nil), path...), k), out)
		}
	case []any:
		for _, child := range t {
			collectCommands(child, path, out)
		}
	}
}

func underEvent(path []string, event string) bool {
	for _, k := range path {
		if strings.EqualFold(k, event) {
			return true
		}
	}
	return false
}

// matchHook reports whether command is the expected hook, and the binary token
// it runs ("" for a wrapper script).
func matchHook(command string, h doctorHook) (bin string, ok bool) {
	tok := strings.Fields(command)
	if len(tok) == 0 {
		return "", false
	}
	base := filepath.Base(tok[0])
	if h.Wrapper != "" && base == h.Wrapper {
		return "", true
	}
	if base != "downshift" && base != "downshift.exe" {
		return "", false
	}
	if strings.Join(tok[1:], " ") != h.Args {
		return "", false
	}
	return tok[0], true
}

type wiredHook struct {
	Spec    doctorHook
	Bin     string
	Command string
}

func hookSnippet(w doctorWiring, h doctorHook, command string) string {
	if w.Nested {
		inner := fmt.Sprintf(`{"type":"command","command":%q}`, command)
		if h.Matcher != "" {
			return fmt.Sprintf(`{"matcher":%q,"hooks":[%s]}`, h.Matcher, inner)
		}
		return fmt.Sprintf(`{"hooks":[%s]}`, inner)
	}
	if h.Matcher != "" {
		return fmt.Sprintf(`{"matcher":%q,"command":%q}`, h.Matcher, command)
	}
	return fmt.Sprintf(`{"command":%q}`, command)
}

func checkHarness(rep *doctorReport, env doctorEnv, nc core.NamedCaps, w doctorWiring, pathBin string, events []telemetry.Event) {
	h := nc.Harness
	file := filepath.Join(env.Home, w.File)
	add := func(id, status, detail, fix string) {
		rep.add(doctorCheck{ID: id, Harness: h, Status: status, Detail: detail, Fix: fix})
	}

	var raw any
	data, err := os.ReadFile(file)
	if err == nil {
		err = json.Unmarshal(data, &raw)
	}
	if err != nil {
		add("hooks wired", doctorFail, fmt.Sprintf("cannot read %s: %v", file, err), "python3 -m json.tool "+file)
		return
	}
	var cmds []foundCommand
	collectCommands(raw, nil, &cmds)

	// Hooks wired.
	var found []wiredHook
	var missing []doctorHook
	for _, spec := range w.Hooks {
		hit := false
		for _, c := range cmds {
			if !underEvent(c.Path, spec.Event) {
				continue
			}
			if bin, ok := matchHook(c.Command, spec); ok {
				found = append(found, wiredHook{Spec: spec, Bin: bin, Command: c.Command})
				hit = true
				break
			}
		}
		if !hit {
			missing = append(missing, spec)
		}
	}
	status, requiredMissing := doctorOK, false
	for _, m := range missing {
		if m.Required {
			requiredMissing = true
		}
	}
	switch {
	case requiredMissing:
		status = doctorFail
	case len(missing) > 0:
		status = doctorWarn
	}
	var have, fixes []string
	for _, f := range found {
		have = append(have, f.Spec.Event+" "+f.Spec.Args)
	}
	binForFix := "downshift"
	if pathBin != "" {
		binForFix = pathBin
	}
	for _, m := range missing {
		command := binForFix + " " + m.Args
		if m.Wrapper != "" {
			command = "/absolute/path/to/scripts/" + m.Wrapper
		}
		fixes = append(fixes, fmt.Sprintf("add to %s under %q: %s", file, m.Event, hookSnippet(w, m, command)))
	}
	detail := fmt.Sprintf("%d of %d expected (%s)", len(found), len(w.Hooks), strings.Join(have, ", "))
	if len(found) == 0 {
		detail = fmt.Sprintf("0 of %d expected in %s", len(w.Hooks), file)
	}
	if len(missing) > 0 {
		var names []string
		for _, m := range missing {
			names = append(names, m.Event+" "+m.Args)
		}
		detail += "; missing: " + strings.Join(names, ", ")
	}
	if w.Setup && len(missing) > 0 {
		fixes = []string{"downshift setup " + h}
	}
	add("hooks wired", status, detail, strings.Join(fixes, "\n"))

	// Hook binary equals the binary on PATH.
	checkHookBinary(add, env, file, found, pathBin)

	// Session list.
	checkSessionList(add, env, nc)

	// Quota snapshot that expired but still holds rewrites.
	checkQuotaHold(add, env, h)

	// Last hook event and rewrite honored, from the shared event log.
	var mine []telemetry.Event
	for _, ev := range events {
		if ev.Harness == h {
			mine = append(mine, ev)
		}
	}
	checkLastEvent(add, env, h, mine)
	checkCompletion(add, h, events)
	if nc.Caps.CanRewriteModel && len(mine) > 0 {
		checkHonored(add, mine)
	}
}

func checkHookBinary(add func(id, status, detail, fix string), env doctorEnv, file string, found []wiredHook, pathBin string) {
	seen := map[string]bool{}
	var bad, fixes []string
	var good string
	for _, f := range found {
		if f.Bin == "" || seen[f.Bin] {
			continue // a wrapper script carries its own binary
		}
		seen[f.Bin] = true
		bin, err := f.Bin, error(nil)
		if !strings.Contains(bin, "/") {
			bin, err = env.LookPath(bin)
		}
		problem := ""
		switch {
		case err != nil:
			problem = f.Bin + " (not found)"
		case !fileExists(bin):
			problem = bin + " (does not exist)"
		case pathBin != "" && resolvedPath(bin) != resolvedPath(pathBin):
			problem = bin
		}
		if problem == "" {
			good = bin
			continue
		}
		bad = append(bad, problem)
		if pathBin != "" {
			fixes = append(fixes, fmt.Sprintf("sed -i.bak 's#%s #%s #g' %s", f.Bin, pathBin, file))
		}
	}
	switch {
	case len(seen) == 0:
	case len(bad) > 0:
		detail := "hooks run " + strings.Join(bad, ", ")
		if pathBin != "" {
			detail += "; PATH has " + pathBin
		} else {
			fixes = append(fixes, "put downshift on PATH (see the binary on PATH check)")
		}
		add("hook binary", doctorFail, detail, strings.Join(fixes, "\n"))
	case pathBin == "":
		add("hook binary", doctorWarn, "hooks run "+good+", but downshift is not on PATH",
			"ln -sf "+good+" "+filepath.Join(env.Home, ".local", "bin", "downshift"))
	default:
		add("hook binary", doctorOK, "hooks run "+good+" (same as PATH)", "")
	}
}

// checkSessionList resolves, with no hook payload, where the harness's session
// model list would come from. A list in the hook payload overrides all of these
// at spawn time.
func checkSessionList(add func(id, status, detail, fix string), env doctorEnv, nc core.NamedCaps) {
	h, now := nc.Harness, env.Now()
	if s, present := core.LoadNativeSession(h, now); present {
		if s.Known {
			add("session list", doctorOK, fmt.Sprintf("native export (%d models)", len(s.IDs)), "")
		} else {
			add("session list", doctorWarn, "native export is set but unreadable; rewrites are held",
				"check "+nc.Caps.NativeSessionEnv+" points at a complete export file")
		}
		return
	}
	if s := core.LoadDiscoveredSession(h, now); s.Known {
		add("session list", doctorOK, fmt.Sprintf("discovered (%d models)", len(s.IDs)), "")
		return
	}
	if s := core.LoadUserSession(h); s.Known {
		add("session list", doctorOK, fmt.Sprintf("file (%d models)", len(s.IDs)), "")
		return
	}
	if nc.Caps.InferSession {
		add("session list", doctorOK, "inferred from the current model (downshift only, never above it)", "")
		return
	}
	fix := fmt.Sprintf("add %q: [\"<model-id>\", ...] to %s", h, core.DefaultSessionModelsPath())
	for _, src := range discovery.Defaults() {
		if src.Harness() == h {
			fix = "downshift models discover --harness=" + h
			break
		}
	}
	add("session list", doctorWarn, "none: no hook list, discovery or file; rewrites are held", fix)
}

// checkQuotaHold warns while an expired quota snapshot still holds rewrites. It
// uses core.StaleQuotaHold, the same rule as the gate: after quota.StaleGrace
// the snapshot is treated as absent and nothing is reported.
func checkQuotaHold(add func(id, status, detail, fix string), env doctorEnv, h string) {
	age, held := core.StaleQuotaHold(h, env.Now())
	if !held {
		return
	}
	left := "held until you refresh it"
	if env.Getenv("DOWNSHIFT_QUOTA_MODE") != "required" {
		left = "rewrites resume on their own in " + humanAge(quota.StaleGrace-age)
	}
	fix := fmt.Sprintf("downshift quota status --harness %s  (refresh the snapshot with the harness statusline bridge or `quota import`; to drop it, delete the %s entry in quota.json)", h, h)
	add("quota snapshot", doctorWarn,
		fmt.Sprintf("expired %s ago and still holding model rewrites; %s", humanAge(age), left), fix)
}

const lastEventStale = 7 * 24 * time.Hour

func checkLastEvent(add func(id, status, detail, fix string), env doctorEnv, h string, mine []telemetry.Event) {
	fix := fmt.Sprintf("restart %s (hooks load at session start), spawn one subagent, then run: downshift stats --days=1", h)
	var last time.Time
	for _, ev := range mine {
		if t, err := time.Parse(time.RFC3339Nano, ev.Timestamp); err == nil && t.After(last) {
			last = t
		}
	}
	if last.IsZero() {
		add("last hook event", doctorWarn, "none in the event log yet", fix)
		return
	}
	age := env.Now().Sub(last)
	if age > lastEventStale {
		add("last hook event", doctorWarn, humanAge(age)+" ago (older than 7 days)", fix)
		return
	}
	add("last hook event", doctorOK, humanAge(age)+" ago", "")
}

// checkCompletion says whether the end of a child spawn is measurable for the
// harness. It is information, not a failure: a harness with no known end event
// simply has no cost per completed task.
func checkCompletion(add func(id, status, detail, fix string), h string, events []telemetry.Event) {
	for _, c := range telemetry.CompletionByHarness(events) {
		if c.Harness == h {
			add("completion", doctorOK, c.Describe(), "")
			return
		}
	}
}

func humanAge(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "under a minute"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

func checkHonored(add func(id, status, detail, fix string), mine []telemetry.Event) {
	shifted, honored := telemetry.CountInferredHonored(mine)
	switch {
	case honored > 0:
		add("rewrite honored", doctorOK, fmt.Sprintf("seen: %d of %d rewrites", honored, shifted), "")
	case shifted > 0:
		add("rewrite honored", doctorWarn, fmt.Sprintf("0 of %d rewrites seen honored; the harness may be ignoring them", shifted),
			"downshift stats --detail  (then see docs/harness-matrix.md)")
	default:
		add("rewrite honored", doctorWarn, "no rewrite emitted yet, so none observed",
			`downshift try "rename a variable"  (shows what would be rewritten), then spawn that task`)
	}
}

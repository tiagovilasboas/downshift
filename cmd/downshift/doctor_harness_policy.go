// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/discovery"
	"github.com/tiagovilasboas/downshift/internal/limits"
	"github.com/tiagovilasboas/downshift/internal/quota"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// Policy, limitations, completion and session status checks for `doctor`.

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
	// A harness that accepts the catalog stand-in (Claude Code on a
	// subscription cannot be listed) still rewrites; discovery only upgrades it.
	if nc.Caps.SessionFromCatalog {
		add("session list", doctorOK, "none cached: the catalog stand-in is used; `models discover` gives a detected list", "")
		return
	}
	// No fixed list is ever configured: a harness with a listing source is
	// discovered; one without gets its list from the session payload only.
	for _, src := range discovery.Defaults() {
		if src.Harness() == h {
			add("session list", doctorWarn, "none: no discovery cache yet; rewrites are held", "downshift models discover --harness="+h)
			return
		}
	}
	add("session list", doctorOK, "from the session payload only (no listing source); rewrites are held until it sends one", "")
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

// checkCompletion says whether the end of a child spawn is measurable for the
// harness. It is information, not a failure: a harness with no known end event
// simply has no tokens per completed task.
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
	t := telemetry.TallyHonored(mine)
	shifted, honored := t.Shifted, t.Honored
	switch {
	case honored > 0:
		add("rewrite honored", doctorOK, fmt.Sprintf("seen: %d of %d rewrites%s", honored, shifted, honorMissDetail(t)), "")
	case shifted > 0:
		add("rewrite honored", doctorWarn, fmt.Sprintf("0 of %d rewrites seen honored; the harness may be ignoring them", shifted),
			"downshift stats --detail  (then see docs/reference/harness-matrix.md)")
	default:
		add("rewrite honored", doctorWarn, "no rewrite emitted yet, so none observed",
			`spawn a subagent task, then run: downshift stats --detail`)
	}
}

// honorMissDetail says why rewrites not seen honored are missing: refuted by
// the child's resolved model, or simply never observed.
func honorMissDetail(t telemetry.HonorTally) string {
	var parts []string
	if t.Refuted > 0 {
		parts = append(parts, fmt.Sprintf("%d refuted: the child ran another model", t.Refuted))
	}
	if n := t.Unobserved(); n > 0 {
		parts = append(parts, fmt.Sprintf("%d not observed: no child record", n))
	}
	if len(parts) == 0 {
		return ""
	}
	return " (" + strings.Join(parts, "; ") + ")"
}

// checkMainPolicy reports the main-session policy setup writes for a harness
// (setupDefault entries with a Note: for Claude Code the model, permissions.defaultMode and You should know defaults).
// A pinned model is a valid choice, so it is reported, not warned about.
func checkMainPolicy(add func(id, status, detail, fix string), harness string, raw any) {
	doc, _ := raw.(map[string]any)
	for _, t := range setupTargets {
		if t.Name != harness {
			continue
		}
		for _, d := range t.Defaults {
			if d.Note == "" {
				continue
			}
			holder := doc
			if d.Parent != "" {
				holder, _ = doc[d.Parent].(map[string]any)
			}
			v, ok := holder[d.Key]
			label := d.Key
			if d.Parent != "" {
				label = d.Parent + "." + d.Key
			}
			if d.Label != "" {
				label = d.Label
			}
			switch {
			case ok && fmt.Sprint(v) == fmt.Sprint(d.Value) && d.Info != "":
				add("main session", doctorOK, label+" "+d.Info, "")
			case ok && fmt.Sprint(v) == fmt.Sprint(d.Value):
				add("main session", doctorOK, fmt.Sprintf("%s %v", label, d.Value), "")
			case ok:
				add("main session", doctorOK, fmt.Sprintf("%s %v (set by you)", label, v), d.Note)
			default:
				add("main session", doctorOK, fmt.Sprintf("no %s set: harness default", label), d.Note)
			}
		}
	}
}

// checkHarnessLimitations reports known limitations of the harness so users
// know what features may not work as expected. This makes the doctor output
// frank about capabilities instead of silently degrading.
func checkHarnessLimitations(add func(id, status, detail, fix string), harness string) {
	// Claude Desktop Chat and Cowork are not hooked harnesses, so they never
	// run the rest of this check on their own id. They are reported here,
	// during the claude-code check, because the Code tab is claude-code and
	// claude-code has full support (no limitations line of its own).
	checkClaudeDesktopSurfaces(add, harness)

	lims := limits.ForHarness(harness)
	if lims == nil {
		return
	}
	unsupported := lims.UnsupportedFeatures()
	if len(unsupported) == 0 {
		return
	}
	// Build a compact summary of limitations
	var parts []string
	for _, lim := range unsupported {
		parts = append(parts, lim.Feature+":"+lim.Status)
	}
	detail := strings.Join(parts, ", ")
	// No command explains harness limitations: `help <harness>` prints hook
	// usage only. The evidence is in docs/reference/harness-matrix.md.
	add("limitations", doctorWarn, detail, "see docs/reference/harness-matrix.md for the evidence")
}

// claudeDesktopSurfaces are the two Claude Desktop surfaces that are not hooked
// harnesses, so they have no hook wiring of their own. doctor reports them
// alongside the claude-code check (the Code tab is the claude-code harness).
var claudeDesktopSurfaces = []string{"claude-desktop-chat", "claude-desktop-cowork"}

// checkClaudeDesktopSurfaces reports, during the claude-code check, what the
// Claude Desktop Chat and Cowork surfaces cannot run. These surfaces have no
// hook of their own, so this is the only place they surface. It runs only when
// Claude Desktop is installed, detected by its app config directory.
func checkClaudeDesktopSurfaces(add func(id, status, detail, fix string), harness string) {
	if harness != "claude-code" || !claudeDesktopInstalled() {
		return
	}
	for _, id := range claudeDesktopSurfaces {
		lims := limits.ForHarness(id)
		if lims == nil {
			continue
		}
		var parts []string
		for _, lim := range lims.Limitations {
			parts = append(parts, lim.Feature+":"+lim.Status)
		}
		add(id, doctorWarn, strings.Join(parts, ", "),
			"see docs/reference/harness-matrix.md for the evidence")
	}
}

// claudeDesktopInstalled reports whether the Claude Desktop app config
// directory is present. OS-agnostic via os.UserConfigDir: on macOS that is
// ~/Library/Application Support/Claude. On an OS where the directory cannot be
// resolved, it reports false (the surfaces are skipped).
func claudeDesktopInstalled() bool {
	dir, err := os.UserConfigDir()
	if err != nil {
		return false
	}
	info, err := os.Stat(filepath.Join(dir, "Claude"))
	return err == nil && info.IsDir()
}

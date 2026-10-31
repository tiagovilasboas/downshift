// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"time"

	"github.com/tiagovilasboas/downshift/internal/quota"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// Proof that the hook ran: the last decision it logged and the quota source
// that gates its rewrites. Both read shared state, so every harness gets them.

const lastEventStale = 7 * 24 * time.Hour

// checkLastEvent reports when the hook last logged and what it decided, so a
// wired hook that never fires is told apart from one that fired and held.
func checkLastEvent(add func(id, status, detail, fix string), env doctorEnv, h string, mine []telemetry.Event) {
	fix := fmt.Sprintf("restart %s (hooks load at session start), spawn one subagent, then run: downshift stats --days=1", h)
	var last time.Time
	var lastDecision telemetry.Event
	var lastDecisionAt time.Time
	for _, ev := range mine {
		t, err := time.Parse(time.RFC3339Nano, ev.Timestamp)
		if err != nil {
			continue
		}
		if t.After(last) {
			last = t
		}
		if isDecision(ev) && t.After(lastDecisionAt) {
			lastDecision, lastDecisionAt = ev, t
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
	detail := humanAge(age) + " ago"
	if !lastDecisionAt.IsZero() {
		detail += "; last decision " + describeDecision(lastDecision, env.Now().Sub(lastDecisionAt))
	}
	add("last hook event", doctorOK, detail, "")
}

// isDecision is true for the events the router wrote when it routed a spawn,
// not usage or baseline rows.
func isDecision(ev telemetry.Event) bool {
	switch ev.Outcome {
	case "usage", "baseline", "":
		return false
	}
	return ev.Verdict != ""
}

func describeDecision(ev telemetry.Event, age time.Duration) string {
	s := fmt.Sprintf("%s %s (%s", humanAge(age)+" ago:", ev.Verdict, ev.Outcome)
	if ev.FromModel != "" && ev.ToModel != "" && ev.FromModel != ev.ToModel {
		s += ", " + ev.FromModel + " -> " + ev.ToModel
	}
	return s + ")"
}

// checkQuotaSource says whether a quota snapshot exists for the harness and
// which source wrote it, e.g. the Claude Code statusline bridge. It is
// information: a missing source never fails, it only means the quota gate has
// no reading to act on.
func checkQuotaSource(add func(id, status, detail, fix string), env doctorEnv, h string) {
	snap, found := quota.Load(h)
	switch {
	case snap != nil:
		src := quota.SourceName(snap.Source)
		if src == "" {
			src = "unrecognised source"
		}
		age := humanAge(env.Now().Sub(snap.ObservedAt))
		if snap.StaleAfterObserved(env.Now()) {
			fix := fmt.Sprintf("a snapshot refreshes only when the harness statusline bridge or `downshift quota import` writes a new one; run `downshift quota status --harness %s` to check", h)
			if src == "claude-statusline" {
				fix = "Claude Code runs the statusline only in an interactive terminal session, so the Claude Desktop Code tab and `claude -p` never refresh it; open an interactive `claude` session to refresh the reading"
			}
			add("quota source", doctorWarn,
				fmt.Sprintf("%s, observed %s ago; older than the %s freshness limit, so quota routing runs on a stale reading", src, age, humanAge(quota.MaxTTL)),
				fix)
			break
		}
		add("quota source", doctorOK, fmt.Sprintf("%s, observed %s ago", src, age), "")
	case found:
		add("quota source", doctorWarn, "quota.json exists but holds no readable snapshot for "+h,
			fmt.Sprintf("downshift quota status --harness %s", h))
	default:
		add("quota source", doctorOK, "none yet; the quota gate has no reading to act on", "")
	}
}

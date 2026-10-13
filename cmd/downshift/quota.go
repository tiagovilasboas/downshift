// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/quota"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// harnessCaps is the capability lookup; tests may substitute it.
var harnessCaps = core.CapsFor

func runQuota(args []string, in io.Reader, out, errOut io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errOut, "usage: downshift quota import|collect|status --harness ID")
		return 2
	}
	if args[0] == "claude-statusline" {
		return runClaudeQuotaStatusline(in, out)
	}
	fs := flag.NewFlagSet("quota "+args[0], flag.ContinueOnError)
	fs.SetOutput(errOut)
	h := fs.String("harness", "", "harness whose quota is observed")
	source := fs.String("source", "", "codex-usage, claude-statusline, cursor-usage, normalized")
	ttl := fs.Duration("ttl", 5*time.Minute, "observation lifetime (maximum 15m)")
	transcript := fs.String("transcript", "", "explicit local Codex transcript path")
	model := fs.String("model", "", "exact model to inspect (status)")
	if fs.Parse(args[1:]) != nil {
		return 2
	}
	if *h == "" || fs.NArg() > 0 {
		fmt.Fprintln(errOut, "quota requires --harness and no positional arguments")
		return 2
	}
	if args[0] == "status" {
		s, present := quota.Load(*h)
		status := quota.Unknown
		if s != nil {
			status = s.Evaluate(*h, *model, time.Now())
		}
		_ = json.NewEncoder(out).Encode(struct {
			Harness  string          `json:"harness"`
			Model    string          `json:"model,omitempty"`
			Status   quota.Status    `json:"status"`
			Present  bool            `json:"present"`
			Snapshot *quota.Snapshot `json:"snapshot,omitempty"`
		}{*h, *model, status, present, s})
		return 0
	}
	if *ttl <= 0 || *ttl > quota.MaxTTL {
		fmt.Fprintln(errOut, "quota ttl must be positive and at most 15m")
		return 2
	}
	var s quota.Snapshot
	var err error
	switch args[0] {
	case "collect":
		if caps, _ := harnessCaps(*h); !caps.TranscriptQuota || *transcript == "" {
			fmt.Fprintln(errOut, "collect requires --harness codex --transcript PATH")
			return 2
		}
		s, err = quota.CollectCodex(*transcript, *ttl)
	case "import":
		b, readErr := io.ReadAll(io.LimitReader(in, (1<<20)+1))
		if readErr != nil || len(b) > 1<<20 {
			fmt.Fprintln(errOut, "quota input exceeds 1MiB or cannot be read")
			return 1
		}
		if *source == "normalized" {
			err = json.Unmarshal(b, &s)
			// Normalized input is operator evidence, never impersonates a native parser.
			s.Source = "operator-normalized"
			if err == nil {
				err = s.Validate()
			}
		} else {
			s, err = quota.ParseNative(*source, b, time.Now().UTC(), *ttl)
		}
	default:
		fmt.Fprintln(errOut, "unknown quota command")
		return 2
	}
	if err != nil {
		fmt.Fprintf(errOut, "quota: %v\n", err)
		return 1
	}
	if s.Harness != *h {
		fmt.Fprintln(errOut, "quota payload belongs to another harness")
		return 1
	}
	if s.ObservedAt.After(time.Now()) {
		fmt.Fprintln(errOut, "quota observation is in the future")
		return 1
	}
	p, err := quota.Path()
	if err == nil {
		err = quota.Store(p, s)
	}
	if err != nil {
		fmt.Fprintf(errOut, "quota: %v\n", err)
		return 1
	}
	_ = json.NewEncoder(out).Encode(s)
	return 0
}

// runClaudeQuotaStatusline is an opt-in native statusLine bridge. It never
// prints raw status JSON and never turns a missing window into availability.
// Operators with an existing statusLine can tee its input to quota import
// instead; this command deliberately does not install or overwrite settings.
//
// It prints the quota windows, then (U1) the routing summary of the current
// session from the tail of the event log. Each part fails on its own: the
// statusline is never an error, and a part with no data is left out.
func runClaudeQuotaStatusline(in io.Reader, out io.Writer) int {
	b, err := io.ReadAll(io.LimitReader(in, (1<<20)+1))
	if err != nil || len(b) > 1<<20 {
		return 0
	}
	line := strings.Join(nonEmpty(statuslineQuota(b), statuslineRouting(b)), " │ ")
	if line != "" {
		fmt.Fprintln(out, line)
	}
	return 0
}

// statuslineQuota stores the snapshot and renders the quota windows ("" on any
// failure).
func statuslineQuota(b []byte) string {
	s, err := quota.ParseNative("claude-statusline", b, time.Now().UTC(), 5*time.Minute)
	if err != nil {
		return ""
	}
	p, err := quota.Path()
	if err != nil || quota.Store(p, s) != nil {
		return ""
	}
	var parts []string
	for i, w := range s.Windows {
		label := "5h"
		if i == 1 {
			label = "7d"
		}
		if w.UsedPercent == nil {
			parts = append(parts, label+" unknown")
		} else {
			parts = append(parts, fmt.Sprintf("%s %.0f%%", label, *w.UsedPercent))
		}
	}
	return strings.Join(parts, " · ")
}

// statuslineRouting renders the routing summary for the statusline's session
// ("" when the session is unknown, the log is missing or unreadable, or the
// session has no routing activity). It reads a bounded tail of the log. While an
// expired quota snapshot still holds rewrites it appends "· quota stale", even
// when the session summary itself is unavailable.
func statuslineRouting(b []byte) (line string) {
	var sum telemetry.SessionRouting
	defer func() {
		if recover() != nil {
			line = ""
		}
	}()
	sum.QuotaStale = statuslineQuotaStale()
	var in struct {
		SessionID string `json:"session_id"`
	}
	if json.Unmarshal(b, &in) != nil {
		return sum.Line()
	}
	hash := telemetry.HashSessionID(in.SessionID)
	if hash == "" {
		return sum.Line()
	}
	events, err := telemetry.ReadEventsTail(telemetry.DefaultTailBytes)
	if err != nil {
		return sum.Line()
	}
	stale := sum.QuotaStale
	sum = telemetry.SessionSummary(events, hash)
	sum.QuotaStale = stale
	return sum.Line()
}

// statuslineQuotaStale is the bridge's one harness fact: its input is Claude
// Code's statusline, so it reads that harness's snapshot.
func statuslineQuotaStale() bool {
	_, held := core.StaleQuotaHold("claude-code", time.Now())
	return held
}

func nonEmpty(parts ...string) []string {
	var out []string
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

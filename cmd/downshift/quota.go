// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/tiagovilasboas/downshift/internal/quota"
)

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
		if *h != "codex" || *transcript == "" {
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
func runClaudeQuotaStatusline(in io.Reader, out io.Writer) int {
	b, err := io.ReadAll(io.LimitReader(in, (1<<20)+1))
	if err != nil || len(b) > 1<<20 {
		return 0
	}
	s, err := quota.ParseNative("claude-statusline", b, time.Now().UTC(), 5*time.Minute)
	if err != nil {
		return 0
	}
	p, err := quota.Path()
	if err != nil || quota.Store(p, s) != nil {
		return 0
	}
	for i, w := range s.Windows {
		if i > 0 {
			fmt.Fprint(out, " · ")
		}
		label := "5h"
		if i == 1 {
			label = "7d"
		}
		if w.UsedPercent == nil {
			fmt.Fprintf(out, "%s unknown", label)
		} else {
			fmt.Fprintf(out, "%s %.0f%%", label, *w.UsedPercent)
		}
	}
	fmt.Fprintln(out)
	return 0
}

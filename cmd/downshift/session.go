// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// runSession is `downshift session` (U3): what Downshift did in one session,
// for the developer to read at a glance. It works for every harness: the
// session comes from --session, else from the environment variable a harness
// declares in HarnessCapabilities.SessionEnv, else it is the most recent session
// in the event log. The text is built in internal/telemetry; this function only
// selects the session and prints.
//
// It reads a bounded tail of the event log and writes nothing except the
// per-session pause marker (--pause / --resume). Every failure to read is an
// empty view, never an error exit.
func runSession(args []string, stdout, stderr io.Writer, catalog core.Resolver) int {
	fs := flag.NewFlagSet("session", flag.ContinueOnError)
	fs.SetOutput(stderr)
	tree := fs.Bool("tree", false, "the session's subagents as a tree (default)")
	why := fs.Bool("why", false, "why the latest decision was made, in plain words")
	pause := fs.Bool("pause", false, "stop rewriting for this session (classify and log only) for up to 12h")
	resume := fs.Bool("resume", false, "undo --pause")
	session := fs.String("session", "", "session id (default: the harness's own, else the most recent in the log)")
	asJSON := fs.Bool("json", false, "tree as JSON")
	panel := fs.Bool("panel", false, "prompt-free per-spawn state and session savings as JSON, for a panel mod")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	modes := 0
	for _, on := range []bool{*tree, *why, *pause, *resume, *panel} {
		if on {
			modes++
		}
	}
	if modes > 1 || (*asJSON && (*why || *pause || *resume || *panel)) || fs.NArg() > 0 {
		fmt.Fprintln(stderr, "usage: downshift session [--tree|--why|--pause|--resume|--panel] [--session=<id>] [--json]  (--json applies to --tree)")
		return 2
	}

	events, _ := telemetry.ReadEventsTail(telemetry.DefaultTailBytes)
	hash, err := selectSession(*session, events)
	if err != nil {
		fmt.Fprintf(stderr, "downshift session: %v\n", err)
		return 2
	}

	switch {
	case *pause, *resume:
		return sessionPauseResume(hash, *pause, stdout, stderr)
	case *why:
		fmt.Fprint(stdout, telemetry.Why(events, hash))
		return 0
	case *panel:
		out, err := json.Marshal(telemetry.BuildPanel(events, hash))
		if err != nil {
			return 1
		}
		fmt.Fprintln(stdout, string(out))
		return 0
	}

	opt := telemetry.TreeOptions{Paused: telemetry.SessionPaused(hash)}
	if *asJSON {
		out, err := json.MarshalIndent(telemetry.BuildSessionTree(events, hash, opt), "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "downshift session: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, string(out))
		return 0
	}
	fmt.Fprint(stdout, telemetry.SessionTree(events, hash, opt))
	return 0
}

func sessionPauseResume(hash string, pause bool, stdout, stderr io.Writer) int {
	if hash == "" {
		fmt.Fprintln(stderr, "downshift session: no session to act on; pass --session=<id>")
		return 1
	}
	short := hash[:8]
	if pause {
		if err := telemetry.PauseSession(hash); err != nil {
			fmt.Fprintf(stderr, "downshift session: could not pause: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "routing paused for session %s for up to 12h: subagents are classified and logged, never rewritten.\nResume with: downshift session --resume\n", short)
		return 0
	}
	if err := telemetry.ResumeSession(hash); err != nil {
		fmt.Fprintf(stderr, "downshift session: could not resume: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "routing resumed for session %s\n", short)
	return 0
}

// selectSession picks the session hash: an explicit id (a raw harness id, a
// full hash, or a unique 8+ hex prefix of a logged hash), else the id in the
// environment variable some harness declares, else the latest in the log. ""
// means no session is known; the view is then empty.
func selectSession(flagVal string, events []telemetry.Event) (string, error) {
	if v := strings.TrimSpace(flagVal); v != "" {
		return sessionFromID(v, events)
	}
	for _, n := range core.AllHarnessCaps() {
		if n.Caps.SessionEnv == "" {
			continue
		}
		if raw := os.Getenv(n.Caps.SessionEnv); raw != "" {
			if h := telemetry.HashSessionID(raw); h != "" {
				return h, nil
			}
		}
	}
	return telemetry.LatestSessionHash(events), nil
}

func sessionFromID(v string, events []telemetry.Event) (string, error) {
	if telemetry.ValidSessionHash(v) {
		return v, nil
	}
	if h := telemetry.HashSessionID(v); h != "" && !isHexPrefix(v) {
		return h, nil
	}
	if isHexPrefix(v) && len(v) >= 8 {
		found := ""
		for _, ev := range events {
			if telemetry.ValidSessionHash(ev.SessionID) && strings.HasPrefix(ev.SessionID, v) {
				if found != "" && found != ev.SessionID {
					return "", fmt.Errorf("session prefix %q is ambiguous", v)
				}
				found = ev.SessionID
			}
		}
		if found != "" {
			return found, nil
		}
	}
	if h := telemetry.HashSessionID(v); h != "" {
		return h, nil
	}
	return "", fmt.Errorf("invalid session id %q", v)
}

func isHexPrefix(v string) bool {
	if v == "" {
		return false
	}
	for _, c := range v {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

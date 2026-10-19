// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// minWatchInterval keeps the tail read of the event log from spinning.
const minWatchInterval = 200 * time.Millisecond

// ANSI: home the cursor and clear the screen. Plain escape codes, no deps.
const ansiClear = "\x1b[H\x1b[2J"

// runWatch is `downshift watch` (U4): the live session tree. It redraws the
// same tree as `downshift session` (telemetry.SessionTree) every interval from
// a bounded tail of the event log, for every harness. It writes nothing and
// exits on Ctrl-C.
func runWatch(args []string, stdout, stderr io.Writer, catalog core.Resolver) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	return watchRun(ctx, args, stdout, stderr, catalog, time.Now, func() []telemetry.Event {
		events, _ := telemetry.ReadEventsTail(telemetry.DefaultTailBytes)
		return events
	})
}

// watchRun is runWatch with its clock, event source and lifetime injected.
func watchRun(ctx context.Context, args []string, stdout, stderr io.Writer, catalog core.Resolver,
	now func() time.Time, read func() []telemetry.Event) int {
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	fs.SetOutput(stderr)
	session := fs.String("session", "", "session id (default: the harness's own, else the most recent in the log)")
	interval := fs.Duration("interval", time.Second, "redraw interval (minimum 200ms)")
	once := fs.Bool("once", false, "draw one frame without clearing the screen, then exit")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintln(stderr, "usage: downshift watch [--session=<id>] [--interval=1s] [--once]")
		return 2
	}
	if *interval < minWatchInterval {
		*interval = minWatchInterval
	}

	frame := func() (string, error) {
		return watchFrame(read(), *session, catalog, now(), *interval)
	}
	if *once {
		out, err := frame()
		if err != nil {
			fmt.Fprintf(stderr, "downshift watch: %v\n", err)
			return 2
		}
		fmt.Fprint(stdout, out)
		return 0
	}
	// An invalid explicit session id fails once, up front.
	if _, err := frame(); err != nil {
		fmt.Fprintf(stderr, "downshift watch: %v\n", err)
		return 2
	}
	tick := time.NewTicker(*interval)
	defer tick.Stop()
	for {
		out, _ := frame()
		fmt.Fprint(stdout, ansiClear, out)
		select {
		case <-ctx.Done():
			fmt.Fprintln(stdout)
			return 0
		case <-tick.C:
		}
	}
}

// watchFrame renders one frame: a status line, then the session tree. With no
// --session it re-selects each frame, so the view follows the newest session.
// A log with no session yet is the empty tree plus a waiting hint, not an error.
func watchFrame(events []telemetry.Event, sessionFlag string, catalog core.Resolver, now time.Time, interval time.Duration) (string, error) {
	hash, err := selectSession(sessionFlag, events)
	if err != nil {
		return "", err
	}
	opt := telemetry.TreeOptions{Paused: telemetry.SessionPaused(hash), Now: now}
	var b strings.Builder
	fmt.Fprintf(&b, "downshift watch · %s · every %s · Ctrl-C to quit\n", now.Format("15:04:05"), interval)
	b.WriteString(telemetry.ActivityLine(events, hash, now) + "\n")
	b.WriteString(telemetry.SessionTree(events, hash, opt))
	if hash == "" {
		b.WriteString("waiting for a session: no subagent has been logged yet\n")
	}
	return b.String(), nil
}

// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/hookport"
)

// runObserver runs an observation command (honor, usage or end) for the one
// harness named in args. A hook runs these, so a wrong argument count prints
// the usage line and exits 0 like every other observation problem: Claude
// Code reads exit 2 from a SubagentStop hook as "keep the subagent running"
// and from PostToolUse as feedback to show the model.
func runObserver(cmd string, args []string, in io.Reader, errW io.Writer, catalog core.Resolver) int {
	if len(args) != 1 {
		fmt.Fprintf(errW, "usage: downshift %s <harness>\n", cmd)
		return 0
	}
	port := portFor(args[0])
	switch cmd {
	case "honor":
		return runHonor(in, port, catalog)
	case "usage":
		return runUsage(in, port, catalog)
	default:
		return runEnd(in, port, catalog)
	}
}

// runHonor is the post-spawn honor runner. The child already started, so any
// read, size, panic, or encode failure prints the neutral response ({}) and
// exits 0. A nil Honor func is unobserved: same stdout, plus one stderr line.
// Telemetry stays inside the port func (DOWNSHIFT_EVENT_LOG respected there).
func runHonor(in io.Reader, port hookport.Port, catalog core.Resolver) (rc int) {
	responded := false
	neutral := func() int {
		if !responded {
			fmt.Println(`{}`)
			responded = true
		}
		return 0
	}
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintln(os.Stderr, "downshift: internal error in honor, ignored")
			rc = neutral()
		}
	}()
	const maxHookPayloadBytes = 1 << 20
	data, err := readHookPayload(in, maxHookPayloadBytes+1)
	if err != nil || len(data) > maxHookPayloadBytes {
		return neutral()
	}
	if owner := foreignPayloadOwner(port.ID, data); owner != "" {
		fmt.Fprint(os.Stderr, foreignPayloadNote(port.ID, owner))
		return neutral()
	}
	obs := hookport.ObserveHonor(port, data, telemetryVersion(), catalog)
	if !obs.Observed {
		fmt.Fprintf(os.Stderr, "downshift: honor unobserved for %s\n", port.ID)
		return neutral()
	}
	encoded, err := json.Marshal(obs.Stdout)
	if err != nil {
		return neutral()
	}
	responded = true
	fmt.Fprintf(os.Stdout, "%s\n", encoded)
	if obs.Note != "" {
		fmt.Fprintf(os.Stderr, "downshift: %s\n", obs.Note)
	}
	return 0
}

// runUsage is the post-spawn usage runner. The child already finished, so it
// never blocks: a read error, an oversized payload, or a panic exits 0 with
// no stdout. A port may return a stdout (the child-end line); otherwise none is
// printed. A nil Usage func is unobserved and prints one stderr line.
func runUsage(in io.Reader, port hookport.Port, catalog core.Resolver) (rc int) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintln(os.Stderr, "downshift: internal error in usage, ignored")
			rc = 0
		}
	}()
	const maxHookPayloadBytes = 1 << 20
	data, err := readHookPayload(in, maxHookPayloadBytes+1)
	if err != nil || len(data) > maxHookPayloadBytes {
		return 0
	}
	if owner := foreignPayloadOwner(port.ID, data); owner != "" {
		fmt.Fprint(os.Stderr, foreignPayloadNote(port.ID, owner))
		return 0
	}
	obs := hookport.ObserveUsage(port, data, telemetryVersion(), catalog)
	if !obs.Observed {
		fmt.Fprintf(os.Stderr, "downshift: usage unobserved for %s\n", port.ID)
		return 0
	}
	if obs.Stdout != nil { // the child-end line, when the port has one to show
		if encoded, err := json.Marshal(obs.Stdout); err == nil {
			fmt.Fprintf(os.Stdout, "%s\n", encoded)
		}
	}
	if obs.Note != "" {
		fmt.Fprintf(os.Stderr, "downshift: %s\n", obs.Note)
	}
	return 0
}

// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// dsmon — downshift monitor
// Floating terminal widget. Lives in cmd/dsmon/ as a standalone command
// inside Downshift — reads events.jsonl, renders live in the terminal.
//
// Build:   go build -o dsmon ./cmd/dsmon
// Run:     ./dsmon
// Float:   ./cmd/dsmon/launch.sh   (opens a floating window)
//
// Standard library only: polling tail on events.jsonl.
package main

import (
	"fmt"
	"github.com/tiagovilasboas/downshift-ai/internal/catalog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/paths"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry/fromcore"
)

// ── config ────────────────────────────────────────────────────────────────────

const (
	maxRecent = 8
	refreshMs = 250 // fast enough to feel live
	boxWidth  = 58
	window24h = 24 * time.Hour
)

// ── main ──────────────────────────────────────────────────────────────────────

func main() {
	fromcore.Install(catalog.Load())
	home, _ := os.UserHomeDir()
	path, err := paths.EventsPath()
	if err != nil {
		path = filepath.Join(home, paths.DirName, "events.jsonl")
	}
	agentPath, err := paths.AgentsPath()
	if err != nil {
		agentPath = filepath.Join(home, paths.DirName, "agents.jsonl")
	}

	fmt.Print(hideCur)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		fmt.Print(showCur + "\n")
		os.Exit(0)
	}()

	fmt.Print(clrScr)
	s := &state{}
	s.reload(path)
	s.pollAgents(agentPath)
	render(s)

	for range time.NewTicker(refreshMs * time.Millisecond).C {
		s.poll(path)
		s.pollAgents(agentPath)
		render(s)
	}
}

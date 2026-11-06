// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/tiagovilasboas/downshift/internal/setup"
)

// The MCP server check for `doctor`, in the claude-code section. It answers one
// question: does the Downshift MCP server work in Claude Code? It reports three
// facts, each a row with its own fix:
//
//   - registered: ~/.claude.json holds mcpServers.downshift (read only; the
//     claude CLI is never started, it would start the servers).
//   - command: the registered command resolves to the running binary.
//   - answers: the registered command, started as `mcp serve`, completes the
//     handshake and lists its tools within a hard timeout.
//   - tool call: the same session calls one read-only tool and gets a valid
//     tool result back (doctor_mcp_call.go).
//
// It only reads files and spawns the registered binary; it never writes
// ~/.claude.json and never runs `claude`. The entry reader is reused from
// internal/setup/mcp.go (setup.ReadClaudeEntry), so the JSON path is not duplicated.

// mcpProbeTimeout caps the whole answers probe: spawn, handshake and tools/list.
const mcpProbeTimeout = 2 * time.Second

// mcpDoctorEnv is what the MCP check reads from the machine, so tests supply a
// temp HOME, a fake running binary and a fake probe.
type mcpDoctorEnv struct {
	// Home is the directory that holds .claude.json.
	Home string
	// Executable reports the running binary (os.Executable in production).
	Executable func() (string, error)
	// Probe starts the registered command as an MCP server, lists its tools and
	// calls one. Tests inject a fake so the real claude CLI and a real server
	// are never needed.
	Probe func(ctx context.Context, command string) (mcpProbeResult, error)
}

// defaultMcpDoctorEnv reads the real machine. HOME comes from os.UserHomeDir so
// a test that sets $HOME is honored, as claudeDesktopInstalled does.
func defaultMcpDoctorEnv() mcpDoctorEnv {
	home, _ := os.UserHomeDir()
	return mcpDoctorEnv{Home: home, Executable: os.Executable, Probe: probeMcpTools}
}

// doctorMcpCheck is the seam the per-harness check calls. It is a package
// variable so a doctor test driven through buildDoctorReport can replace the
// machine-reading check (which spawns a server and reads the real
// ~/.claude.json) with a controlled fake; production uses checkClaudeCodeMcp.
var doctorMcpCheck = checkClaudeCodeMcp

// checkClaudeCodeMcp runs the MCP check during the claude-code harness check.
// It runs once, in the claude-code section, and also lists the other targets.
func checkClaudeCodeMcp(add func(id, status, detail, fix string), harness string) {
	if harness != "claude-code" {
		return
	}
	env := defaultMcpDoctorEnv()
	checkClaudeCodeMcpWith(add, env)
	// The file-based targets setup registers an MCP entry for (doctor_mcp_targets.go).
	checkMcpTargets(add, env.Home, setup.Targets)
}

// mcpSetupFix is the one fix that repairs a missing or mismatched registration.
const mcpSetupFix = "downshift setup --mcp"

// checkClaudeCodeMcpWith is the testable core. It reports the three MCP rows.
// A failure is a warning, never a failure: the MCP server is an extra on top of
// the routing hooks, so it must not turn doctor red.
func checkClaudeCodeMcpWith(add func(id, status, detail, fix string), env mcpDoctorEnv) {
	file := filepath.Join(env.Home, ".claude.json")
	entry, err := setup.ReadClaudeEntry(file)
	switch {
	case err != nil:
		add("mcp registered", doctorWarn, fmt.Sprintf("%s unreadable: %v", file, err), mcpSetupFix)
		return
	case !entry.Found:
		add("mcp registered", doctorWarn, "no mcpServers.downshift in "+file, mcpSetupFix)
		return
	case !entry.Valid:
		add("mcp registered", doctorWarn, "mcpServers.downshift is not a command entry in "+file, mcpSetupFix)
		return
	}
	add("mcp registered", doctorOK, "mcpServers.downshift runs "+entry.Command, "")

	if !checkMcpCommand(add, env, entry.Command) {
		// A command that does not resolve to a real binary cannot be probed;
		// the command row already carries the fix, so stop here.
		return
	}
	checkMcpAnswers(add, env, entry.Command)
}

// checkMcpCommand warns when the registered command and the running binary
// differ once symlinks are resolved. It returns false when the registered
// command cannot be resolved to a real file, so the answers probe is skipped.
func checkMcpCommand(add func(id, status, detail, fix string), env mcpDoctorEnv, command string) bool {
	regReal, regErr := filepath.EvalSymlinks(command)
	if regErr != nil {
		add("mcp command", doctorWarn, "registered command does not resolve: "+command, mcpSetupFix)
		return false
	}
	exe, _ := env.Executable()
	runReal := exe
	if r, err := filepath.EvalSymlinks(exe); err == nil {
		runReal = r
	}
	if runReal != "" && regReal != runReal {
		add("mcp command", doctorWarn,
			fmt.Sprintf("registered %s resolves to %s; this binary is %s", command, regReal, runReal),
			mcpSetupFix)
		return true
	}
	add("mcp command", doctorOK, "registered command is this binary ("+regReal+")", "")
	return true
}

// checkMcpAnswers starts the registered command as an MCP server, runs the
// handshake, lists its tools and reports the count. A hard timeout bounds the
// whole probe and the child is killed on every path (probeMcpTools).
func checkMcpAnswers(add func(id, status, detail, fix string), env mcpDoctorEnv, command string) {
	ctx, cancel := context.WithTimeout(context.Background(), mcpProbeTimeout)
	defer cancel()
	res, err := env.Probe(ctx, command)
	if err != nil {
		add("mcp answers", doctorWarn, "the server did not answer: "+err.Error(), mcpSetupFix)
		return
	}
	add("mcp answers", doctorOK, fmt.Sprintf("%d tools listed over stdio", res.Tools), "")
	if res.CallErr != nil {
		add("mcp tool call", doctorWarn, "the server listed its tools but "+res.CallErr.Error(), mcpSetupFix)
		return
	}
	add("mcp tool call", doctorOK, mcpCallTool+" returned a valid tool result", "")
}

// probeMcpTools starts `<command> mcp serve`, completes the initialize
// handshake, asks for the tool list and calls one tool. The transport is
// newline-delimited JSON-RPC (internal/mcp/transport). The child is killed when
// ctx ends or this returns, on every path. Standard error is discarded: the
// server logs there, only standard output carries protocol messages.
func probeMcpTools(ctx context.Context, command string) (mcpProbeResult, error) {
	cmd := exec.CommandContext(ctx, command, "mcp", "serve")
	cmd.Stderr = nil
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return mcpProbeResult{}, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return mcpProbeResult{}, err
	}
	if err := cmd.Start(); err != nil {
		return mcpProbeResult{}, err
	}
	// Kill the child on every path and reap it, so no server is left running.
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	if err := writeMcpHandshake(stdin); err != nil {
		return mcpProbeResult{}, err
	}
	return readMcpReplies(ctx, stdout)
}

// mcpInitID and mcpListID are the request ids used by the probe; the tools/list
// reply is matched by mcpListID. The tools/call id is mcpCallID.
const (
	mcpInitID = 1
	mcpListID = 2
)

// writeMcpHandshake sends initialize, the initialized notification and
// tools/list and tools/call as newline-delimited JSON-RPC. The writer stays
// open until the replies are read: the probe kills the child when it is done.
func writeMcpHandshake(w interface {
	Write([]byte) (int, error)
	Close() error
}) error {
	msgs := []map[string]any{
		{"jsonrpc": "2.0", "id": mcpInitID, "method": "initialize", "params": map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "downshift-doctor", "version": buildVersion},
		}},
		{"jsonrpc": "2.0", "method": "notifications/initialized"},
		{"jsonrpc": "2.0", "id": mcpListID, "method": "tools/list"},
		mcpCallRequest(),
	}
	for _, m := range msgs {
		line, err := json.Marshal(m)
		if err != nil {
			return err
		}
		if _, err := w.Write(append(line, '\n')); err != nil {
			return err
		}
	}
	return nil
}

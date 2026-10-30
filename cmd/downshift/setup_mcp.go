// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// `downshift setup` registers `downshift mcp serve` as the MCP server
// "downshift" in the harnesses that declare setupTarget.Mcp. It is on by
// default; `--no-mcp` skips it, and every write is shown first. Claude Code is registered by
// its own CLI (setup_mcp.go); Codex by a marked TOML block (setup_mcp_toml.go);
// `--undo` removes exactly that (setup_mcp_undo.go). Facts and the list of what
// is unverified: docs/internal/plans/mcp-registration.md.

const mcpServerName = "downshift"

// mcpServeArgs are the arguments after the binary that start the server.
var mcpServeArgs = []string{"mcp", "serve"}

// mcpClaudeStep is a Claude Code CLI call setup still has to make.
type mcpClaudeStep struct {
	target setupTarget
	claude string   // the CLI path found on PATH
	args   []string // arguments after the CLI path
	shown  string   // the command as printed
}

// planMcp plans the MCP registration of every selected target that supports
// it. File edits come back as plans for applyPlans; CLI calls come back as
// steps for runMcpSteps. A conflict or an unreadable file is a warning: it is
// left as it is and setup goes on, because the MCP server is an extra that must
// not block the hooks. With --no-mcp (Mcp false) it does nothing.
func planMcp(o setupOptions, targets []setupTarget) (files []setupPlan, steps []mcpClaudeStep) {
	if !o.Mcp {
		return nil, nil
	}
	for _, t := range targets {
		if t.Mcp == nil {
			continue
		}
		switch t.Mcp.Kind {
		case mcpViaCLI:
			if step, ok := planClaudeMcp(o, t); ok {
				steps = append(steps, step)
			}
		case mcpTOMLBlock:
			if p, ok := planCodexMcp(o, t); ok {
				files = append(files, p)
			}
		case mcpJSONMap:
			if p, ok := planMcpJSON(o, t); ok {
				files = append(files, p)
			}
		}
	}
	return files, steps
}

// mcpEntry is what a harness file holds under the server name.
type mcpEntry struct {
	found   bool
	command string
	args    []string
	valid   bool // false when the entry is not a command/args object
	// managed is true when the entry carries the env marker setup puts on the
	// entry it adds; undo and "up to date" require it, so a hand-made entry that
	// merely runs `mcp serve` is never touched.
	managed bool
}

// The env marker setup adds to the Claude Code entry.
const (
	mcpMarkerKey   = "DOWNSHIFT_MANAGED"
	mcpMarkerValue = "setup"
)

func (e mcpEntry) matches(bin string) bool {
	return e.valid && e.managed && e.command == bin && mcpSameArgs(e.args, mcpServeArgs)
}

func mcpSameArgs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// readClaudeEntry reads mcpServers.downshift from the Claude Code user file.
// It only reads: `claude mcp list` and `get` would start the servers.
func readClaudeEntry(path string) (mcpEntry, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return mcpEntry{}, nil
	}
	if err != nil {
		return mcpEntry{}, err
	}
	var doc struct {
		McpServers map[string]json.RawMessage `json:"mcpServers"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return mcpEntry{}, errors.New("not valid JSON, left untouched")
	}
	entry, ok := doc.McpServers[mcpServerName]
	if !ok {
		return mcpEntry{}, nil
	}
	var e struct {
		Command string            `json:"command"`
		Args    []string          `json:"args"`
		Env     map[string]string `json:"env"`
	}
	if json.Unmarshal(entry, &e) != nil || e.Command == "" {
		return mcpEntry{found: true}, nil
	}
	return mcpEntry{found: true, valid: true, command: e.Command, args: e.Args, managed: e.Env[mcpMarkerKey] == mcpMarkerValue}, nil
}

// claudeCLI finds the claude CLI through the injected LookPath. The zero
// setupExec finds nothing, so a caller that sets no Exec never runs a CLI.
func claudeCLI(o setupOptions) (string, bool) {
	if o.Exec.LookPath == nil || o.Exec.Run == nil {
		return "", false
	}
	p, err := o.Exec.LookPath("claude")
	return p, err == nil
}

// mcpShown renders a command line for the report, quoting what has a space.
func mcpShown(parts ...string) string {
	out := make([]string, len(parts))
	for i, p := range parts {
		if strings.ContainsAny(p, " \t") {
			p = `"` + p + `"`
		}
		out[i] = p
	}
	return strings.Join(out, " ")
}

func planClaudeMcp(o setupOptions, t setupTarget) (step mcpClaudeStep, ok bool) {
	file := filepath.Join(o.Home, filepath.FromSlash(t.Mcp.File))
	shown := setupDisplay(o.Home, file)
	cli, have := claudeCLI(o)
	if !have {
		fmt.Fprintf(o.Out, "%s: MCP server not registered: `claude` is not on PATH (skipped; install Claude Code and rerun `downshift setup`)\n\n", t.Name)
		return step, false
	}
	entry, err := readClaudeEntry(file)
	if err != nil {
		fmt.Fprintf(o.Err, "warning: %s: %s: %v\n  the MCP server was not registered; the file was left as it is\n\n", t.Name, shown, err)
		return step, false
	}
	switch {
	case entry.matches(o.Binary):
		fmt.Fprintf(o.Out, "%s: MCP server %q already up to date (%s)\n\n", t.Name, mcpServerName, shown)
		return step, false
	case entry.found:
		fmt.Fprintf(o.Err, "warning: %s: %s already has an MCP server named %q that setup did not add (other command or no setup marker); "+
			"it was left as it is. Remove it (`claude mcp remove -s user %s`) and rerun setup to let Downshift register it\n\n",
			t.Name, shown, mcpServerName, mcpServerName)
		return step, false
	}
	args := append([]string{"mcp", "add", "-s", "user", mcpServerName, "-e", mcpMarkerKey + "=" + mcpMarkerValue, "--", o.Binary}, mcpServeArgs...)
	step = mcpClaudeStep{target: t, claude: cli, args: args, shown: "claude " + mcpShown(args...)}
	verb := "will run"
	if o.DryRun {
		verb = "would run"
	}
	fmt.Fprintf(o.Out, "%s: %s: %s\n\n", t.Name, verb, step.shown)
	return step, true
}

// runSetupMcp writes the planned config edits and runs the planned CLI calls.
// confirmed means the user already said yes to the other file writes of this
// run, so it is not asked again; failed means those writes failed or were
// declined, which also skips this step. Their diffs and commands were printed
// by planMcp.
func runSetupMcp(o setupOptions, files []setupPlan, steps []mcpClaudeStep, confirmed, failed bool) (bad bool) {
	if len(files)+len(steps) == 0 {
		return false
	}
	if o.DryRun {
		fmt.Fprintln(o.Out, "Dry run: no MCP registration was written.")
		return false
	}
	if !confirmed {
		if failed {
			return false
		}
		if !setupConfirm(o, "Register the MCP server as shown above? A backup of each existing file is kept. [y/N] ") {
			fmt.Fprintln(o.Out, "Not applied. Re-run with --yes to skip this question.")
			return true
		}
	}
	stamp := o.Now().UTC().Format("20060102T150405.000000")
	for _, p := range files {
		bak, err := setupWrite(p, stamp)
		switch {
		case err != nil:
			fmt.Fprintf(o.Err, "%s: write failed: %v\n", p.target.Name, err)
			bad = true
		case bak != "":
			fmt.Fprintf(o.Out, "%s: written. Backup: %s\n", p.target.Name, setupDisplay(o.Home, bak))
		default:
			fmt.Fprintf(o.Out, "%s: written (new file).\n", p.target.Name)
		}
	}
	for _, s := range steps {
		if out, err := o.Exec.Run(s.claude, s.args...); err != nil {
			fmt.Fprintf(o.Err, "%s: `%s` failed: %v\n%s\n", s.target.Name, s.shown, err, mcpCLIOutput(out))
			bad = true
			continue
		}
		file := filepath.Join(o.Home, filepath.FromSlash(s.target.Mcp.File))
		if e, err := readClaudeEntry(file); err != nil || !e.matches(o.Binary) {
			fmt.Fprintf(o.Err, "%s: the CLI reported success but %s does not hold the %q entry\n",
				s.target.Name, setupDisplay(o.Home, file), mcpServerName)
			bad = true
			continue
		}
		fmt.Fprintf(o.Out, "%s: MCP server %q registered (%s)\n", s.target.Name, mcpServerName, setupDisplay(o.Home, file))
	}
	return bad
}

// mcpCLIOutput keeps the first lines of a failing CLI's output.
func mcpCLIOutput(out []byte) string {
	text := strings.TrimSpace(string(out))
	lines := strings.Split(text, "\n")
	if len(lines) > 5 {
		lines = lines[:5]
	}
	text = strings.Join(lines, "\n")
	if len(text) > 400 {
		text = text[:400]
	}
	return text
}

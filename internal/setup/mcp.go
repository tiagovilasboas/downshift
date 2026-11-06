// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// `downshift setup` registers `downshift mcp serve` as the MCP server
// "downshift" in the harnesses that declare Target.Mcp. It is on by
// default; `--no-mcp` skips it, and every write is shown first. Claude Code is registered by
// its own CLI (mcp.go); Codex by a marked TOML block (mcp_toml.go);
// `--undo` removes exactly that (mcp_undo.go). Facts and the list of what
// is unverified: docs/internal/plans/mcp-registration.md.

// mcpClaudeStep is a Claude Code CLI call setup still has to make.
type mcpClaudeStep struct {
	target Target
	claude string   // the CLI path found on PATH
	args   []string // arguments after the CLI path
	shown  string   // the command as printed
}

// planMcp plans the MCP registration of every selected target that supports
// it. File edits come back as plans for ApplyPlans; CLI calls come back as
// steps for runMcpSteps. A conflict or an unreadable file is a warning: it is
// left as it is and setup goes on, because the MCP server is an extra that must
// not block the hooks. With --no-mcp (Mcp false) it does nothing.
func planMcp(o Options, targets []Target) (files []Plan, steps []mcpClaudeStep) {
	if !o.Mcp {
		return nil, nil
	}
	for _, t := range targets {
		if t.Mcp == nil {
			continue
		}
		switch t.Mcp.Kind {
		case McpViaCLI:
			if step, ok := planClaudeMcp(o, t); ok {
				steps = append(steps, step)
			}
		case McpTOMLBlock:
			if p, ok := PlanCodexMcp(o, t); ok {
				files = append(files, p)
			}
		case McpJSONMap:
			if p, ok := PlanMcpJSON(o, t); ok {
				files = append(files, p)
			}
		}
	}
	return files, steps
}

// McpEntry is what a harness file holds under the server name.
type McpEntry struct {
	Found   bool
	Command string
	args    []string
	Valid   bool // false when the entry is not a command/args object
	// managed is true when the entry carries the env marker setup puts on the
	// entry it adds; undo and "up to date" require it, so a hand-made entry that
	// merely runs `mcp serve` is never touched.
	managed bool
}

func (e McpEntry) matches(bin string) bool {
	return e.Valid && e.managed && e.Command == bin && McpSameArgs(e.args, McpServeArgs)
}

// ReadClaudeEntry reads mcpServers.downshift from the Claude Code user file.
// It only reads: `claude mcp list` and `get` would start the servers.
func ReadClaudeEntry(path string) (McpEntry, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return McpEntry{}, nil
	}
	if err != nil {
		return McpEntry{}, err
	}
	var doc struct {
		McpServers map[string]json.RawMessage `json:"mcpServers"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return McpEntry{}, errors.New("not valid JSON, left untouched")
	}
	entry, ok := doc.McpServers[McpServerName]
	if !ok {
		return McpEntry{}, nil
	}
	var e struct {
		Command string            `json:"command"`
		Args    []string          `json:"args"`
		Env     map[string]string `json:"env"`
	}
	if json.Unmarshal(entry, &e) != nil || e.Command == "" {
		return McpEntry{Found: true}, nil
	}
	return McpEntry{Found: true, Valid: true, Command: e.Command, args: e.Args, managed: e.Env[McpMarkerKey] == McpMarkerValue}, nil
}

// claudeCLI finds the claude CLI through the injected LookPath. The zero
// Exec finds nothing, so a caller that sets no Exec never runs a CLI.
func claudeCLI(o Options) (string, bool) {
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

func planClaudeMcp(o Options, t Target) (step mcpClaudeStep, ok bool) {
	file := filepath.Join(o.Home, filepath.FromSlash(t.Mcp.File))
	shown := Display(o.Home, file)
	cli, have := claudeCLI(o)
	if !have {
		fmt.Fprintf(o.Out, "%s: MCP server not registered: `claude` is not on PATH (skipped; install Claude Code and rerun `downshift setup`)\n\n", t.Name)
		return step, false
	}
	entry, err := ReadClaudeEntry(file)
	if err != nil {
		fmt.Fprintf(o.Err, "warning: %s: %s: %v\n  the MCP server was not registered; the file was left as it is\n\n", t.Name, shown, err)
		return step, false
	}
	switch {
	case entry.matches(o.Binary):
		fmt.Fprintf(o.Out, "%s: MCP server %q already up to date (%s)\n\n", t.Name, McpServerName, shown)
		return step, false
	case entry.Found:
		fmt.Fprintf(o.Err, "warning: %s: %s already has an MCP server named %q that setup did not add (other command or no setup marker); "+
			"it was left as it is. Remove it (`claude mcp remove -s user %s`) and rerun setup to let Downshift register it\n\n",
			t.Name, shown, McpServerName, McpServerName)
		return step, false
	}
	args := append([]string{"mcp", "add", "-s", "user", McpServerName, "-e", McpMarkerKey + "=" + McpMarkerValue, "--", o.Binary}, McpServeArgs...)
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
func runSetupMcp(o Options, files []Plan, steps []mcpClaudeStep, confirmed, failed bool) (bad bool) {
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
		if !Confirm(o, "Register the MCP server as shown above? A backup of each existing file is kept. [y/N] ") {
			fmt.Fprintln(o.Out, "Not applied. Re-run with --yes to skip this question.")
			return true
		}
	}
	stamp := o.Now().UTC().Format("20060102T150405.000000")
	for _, p := range files {
		p.Bases = o.LinkBases()
		bak, err := Write(p, stamp)
		switch {
		case err != nil:
			fmt.Fprintf(o.Err, "%s: write failed: %v\n", p.Target.Name, err)
			bad = true
			continue
		case bak != "":
			fmt.Fprintf(o.Out, "%s: %s written. Backup: %s\n", p.Target.Name, Display(o.Home, p.Path), Display(o.Home, bak))
		default:
			fmt.Fprintf(o.Out, "%s: %s written (new file).\n", p.Target.Name, Display(o.Home, p.Path))
		}
		// A JSON MCP-only target (Claude Desktop) carries its after-write notes
		// here: the hook/skills flow that normally prints Notes never runs for it.
		if p.Target.File == "" {
			PrintNotes(o.Out, p.Target.Notes)
		}
		setupWarnOwnerApp(o, p.Target)
	}
	for _, s := range steps {
		if out, err := o.Exec.Run(s.claude, s.args...); err != nil {
			fmt.Fprintf(o.Err, "%s: `%s` failed: %v\n%s\n", s.target.Name, s.shown, err, mcpCLIOutput(out))
			bad = true
			continue
		}
		file := filepath.Join(o.Home, filepath.FromSlash(s.target.Mcp.File))
		if e, err := ReadClaudeEntry(file); err != nil || !e.matches(o.Binary) {
			fmt.Fprintf(o.Err, "%s: the CLI reported success but %s does not hold the %q entry\n",
				s.target.Name, Display(o.Home, file), McpServerName)
			bad = true
			continue
		}
		fmt.Fprintf(o.Out, "%s: MCP server %q registered (%s)\n", s.target.Name, McpServerName, Display(o.Home, file))
	}
	return bad
}

// setupWarnOwnerApp says, after a write, when the app that owns the file will
// drop the entry: a warning on stderr when it is running, a note when the OS
// cannot tell, nothing when it is known to be stopped.
func setupWarnOwnerApp(o Options, t Target) {
	if t.Mcp == nil || t.Mcp.OwnerApp == "" {
		return
	}
	state := OwnerAppRunningState(t.Mcp.OwnerApp)
	msg := ownerAppWarning(t.Mcp.OwnerApp, state)
	switch state {
	case OwnerAppStopped:
	case OwnerAppRunning:
		fmt.Fprintf(o.Err, "%s: warning: %s\n", t.Name, msg)
	default:
		PrintNotes(o.Out, []string{msg})
	}
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

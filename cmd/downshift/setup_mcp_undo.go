// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// mcpUndo removes the MCP registrations setup --mcp added, and only those: the
// marked Codex block, and the Claude Code entry named downshift that runs
// `mcp serve`. Other servers and every byte outside the block stay. It is
// silent for a harness with nothing to remove, so a plain `setup --undo` of an
// install without --mcp prints exactly what it printed before.
func mcpUndo(o setupOptions, targets []setupTarget) int {
	var edits []setupPlan
	var steps []mcpClaudeStep
	code := 0
	for _, t := range targets {
		if t.Mcp == nil {
			continue
		}
		switch t.Mcp.Kind {
		case mcpViaCLI:
			step, ok, bad := planClaudeMcpUndo(o, t)
			if bad {
				code = 1
			}
			if ok {
				steps = append(steps, step)
			}
		case mcpTOMLBlock:
			p, ok, bad := planCodexMcpUndo(o, t)
			if bad {
				code = 1
			}
			if ok {
				edits = append(edits, p)
			}
		}
	}
	if len(edits)+len(steps) == 0 {
		return code
	}
	verb := "will"
	if o.DryRun {
		verb = "would"
	}
	for _, p := range edits {
		shown := setupDisplay(o.Home, p.path)
		fmt.Fprintf(o.Out, "%s: %s %s remove the downshift MCP block\n", p.target.Name, shown, verb)
		fmt.Fprint(o.Out, mcpBlockDiff(shown, true, string(p.before), string(p.after)))
		fmt.Fprintln(o.Out)
	}
	for _, s := range steps {
		fmt.Fprintf(o.Out, "%s: %s run: %s\n\n", s.target.Name, verb, s.shown)
	}
	if o.DryRun {
		fmt.Fprintln(o.Out, "Dry run: nothing was written.")
		return code
	}
	if !setupConfirm(o, "Remove the MCP registration? [y/N] ") {
		fmt.Fprintln(o.Out, "Not applied. Re-run with --yes to skip this question.")
		return 1
	}
	stamp := o.Now().UTC().Format("20060102T150405.000000")
	for _, p := range edits {
		if err := writeMcpUndo(p, stamp); err != nil {
			fmt.Fprintf(o.Err, "%s: remove failed: %v\n", p.target.Name, err)
			code = 1
			continue
		}
		fmt.Fprintf(o.Out, "%s: removed the downshift MCP block from %s\n", p.target.Name, setupDisplay(o.Home, p.path))
	}
	for _, s := range steps {
		if out, err := o.Exec.Run(s.claude, s.args...); err != nil {
			fmt.Fprintf(o.Err, "%s: `%s` failed: %v\n%s\n", s.target.Name, s.shown, err, mcpCLIOutput(out))
			code = 1
			continue
		}
		file := filepath.Join(o.Home, filepath.FromSlash(s.target.Mcp.File))
		if e, err := readClaudeEntry(file); err != nil || e.found {
			fmt.Fprintf(o.Err, "%s: the CLI reported success but the %q entry is still in %s\n",
				s.target.Name, mcpServerName, setupDisplay(o.Home, file))
			code = 1
			continue
		}
		fmt.Fprintf(o.Out, "%s: MCP server %q removed\n", s.target.Name, mcpServerName)
	}
	return code
}

// writeMcpUndo writes the block-free text with the usual backup. A file that
// holds nothing else is removed, since setup created it.
func writeMcpUndo(p setupPlan, stamp string) error {
	if _, err := setupWrite(p, stamp); err != nil {
		return err
	}
	if strings.TrimSpace(string(p.after)) == "" {
		if err := os.Remove(p.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func planCodexMcpUndo(o setupOptions, t setupTarget) (p setupPlan, ok, failed bool) {
	path := filepath.Join(o.Home, filepath.FromSlash(t.Mcp.File))
	raw, err := os.ReadFile(path)
	if err != nil {
		return p, false, false // no file, no block
	}
	if _, _, found, err := mcpBlockBounds(string(raw)); err != nil || !found {
		if err != nil {
			fmt.Fprintf(o.Err, "%s: %s: %v\n", t.Name, setupDisplay(o.Home, path), err)
		}
		return p, false, err != nil
	}
	return setupPlan{target: t, path: path, existed: true, before: raw, after: []byte(removeMcpBlock(string(raw)))}, true, false
}

func planClaudeMcpUndo(o setupOptions, t setupTarget) (step mcpClaudeStep, ok, failed bool) {
	file := filepath.Join(o.Home, filepath.FromSlash(t.Mcp.File))
	entry, err := readClaudeEntry(file)
	if err != nil || !entry.found {
		return step, false, false // unreadable or absent: setup added nothing there
	}
	if !entry.valid || !mcpSameArgs(entry.args, mcpServeArgs) || !entry.managed {
		fmt.Fprintf(o.Out, "%s: the MCP server %q in %s was not registered by setup, left as it is\n\n",
			t.Name, mcpServerName, setupDisplay(o.Home, file))
		return step, false, false
	}
	cli, have := claudeCLI(o)
	if !have {
		fmt.Fprintf(o.Out, "%s: the MCP server %q is registered, but `claude` is not on PATH; remove it with `claude mcp remove -s user %s`\n\n",
			t.Name, mcpServerName, mcpServerName)
		return step, false, false
	}
	args := []string{"mcp", "remove", "-s", "user", mcpServerName}
	return mcpClaudeStep{target: t, claude: cli, args: args, shown: "claude " + mcpShown(args...)}, true, false
}

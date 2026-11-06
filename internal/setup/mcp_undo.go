// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// mcpUndo removes the MCP registrations setup --mcp added, and only those: the
// marked Codex block, and the Claude Code entry named downshift that runs
// `mcp serve`. Other servers and every byte outside the block stay. It is
// silent for a harness with nothing to remove, so a plain `setup --undo` of an
// install without --mcp prints exactly what it printed before.
func mcpUndo(o Options, targets []Target) int {
	var edits []Plan
	var steps []mcpClaudeStep
	code := 0
	for _, t := range targets {
		if t.Mcp == nil {
			continue
		}
		switch t.Mcp.Kind {
		case McpViaCLI:
			step, ok, bad := planClaudeMcpUndo(o, t)
			if bad {
				code = 1
			}
			if ok {
				steps = append(steps, step)
			}
		case McpTOMLBlock:
			p, ok, bad := planCodexMcpUndo(o, t)
			if bad {
				code = 1
			}
			if ok {
				edits = append(edits, p)
			}
		case McpJSONMap:
			p, ok, bad := PlanMcpJSONUndo(o, t)
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
		shown := Display(o.Home, p.Path)
		if o.Out != nil {
			fmt.Fprintf(o.Out, "%s: %s %s remove the downshift MCP block\n", p.Target.Name, shown, verb)
			fmt.Fprint(o.Out, McpBlockDiff(shown, true, string(p.Before), string(p.After)))
			fmt.Fprintln(o.Out)
		}
	}
	for _, s := range steps {
		if o.Out != nil {
			fmt.Fprintf(o.Out, "%s: %s run: %s\n\n", s.target.Name, verb, s.shown)
		}
	}
	if o.DryRun {
		if o.Out != nil {
			fmt.Fprintln(o.Out, "Dry run: nothing was written.")
		}
		return code
	}
	if !Confirm(o, "Remove the MCP registration? [y/N] ") {
		if o.Out != nil {
			fmt.Fprintln(o.Out, "Not applied. Re-run with --yes to skip this question.")
		}
		return 1
	}
	now := time.Now
	if o.Now != nil {
		now = o.Now
	}
	stamp := now().UTC().Format("20060102T150405.000000")
	for _, p := range edits {
		p.Bases = o.LinkBases()
		if err := writeMcpUndo(p, stamp); err != nil {
			if o.Err != nil {
				fmt.Fprintf(o.Err, "%s: remove failed: %v\n", p.Target.Name, err)
			}
			code = 1
			continue
		}
		if o.Out != nil {
			fmt.Fprintf(o.Out, "%s: removed the downshift MCP block from %s\n", p.Target.Name, Display(o.Home, p.Path))
		}
	}
	for _, s := range steps {
		if out, err := o.Exec.Run(s.claude, s.args...); err != nil {
			if o.Err != nil {
				fmt.Fprintf(o.Err, "%s: `%s` failed: %v\n%s\n", s.target.Name, s.shown, err, mcpCLIOutput(out))
			}
			code = 1
			continue
		}
		file := filepath.Join(o.Home, filepath.FromSlash(s.target.Mcp.File))
		if e, err := ReadClaudeEntry(file); err != nil || e.Found {
			if o.Err != nil {
				fmt.Fprintf(o.Err, "%s: the CLI reported success but the %q entry is still in %s\n",
					s.target.Name, McpServerName, Display(o.Home, file))
			}
			code = 1
			continue
		}
		if o.Out != nil {
			fmt.Fprintf(o.Out, "%s: MCP server %q removed\n", s.target.Name, McpServerName)
		}
	}
	return code
}

// writeMcpUndo writes the block-free text with the usual backup. A file that
// holds nothing else is removed, since setup created it.
func writeMcpUndo(p Plan, stamp string) error {
	if _, err := Write(p, stamp); err != nil {
		return err
	}
	if strings.TrimSpace(string(p.After)) == "" {
		if err := os.Remove(p.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func planCodexMcpUndo(o Options, t Target) (p Plan, ok, failed bool) {
	path := filepath.Join(o.Home, filepath.FromSlash(t.Mcp.File))
	if err := ErrLinkedPath(path, o.LinkBases()...); err != nil {
		if o.Err != nil {
			fmt.Fprintf(o.Err, "%s: %s: %v\n", t.Name, Display(o.Home, path), err)
		}
		return p, false, true
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return p, false, false // no file, no block
	}
	if _, _, found, err := McpBlockBounds(string(raw)); err != nil || !found {
		if err != nil {
			if o.Err != nil {
				fmt.Fprintf(o.Err, "%s: %s: %v\n", t.Name, Display(o.Home, path), err)
			}
		}
		return p, false, err != nil
	}
	return Plan{Target: t, Path: path, Existed: true, Before: raw, After: []byte(RemoveMcpBlock(string(raw)))}, true, false
}

func planClaudeMcpUndo(o Options, t Target) (step mcpClaudeStep, ok, failed bool) {
	file := filepath.Join(o.Home, filepath.FromSlash(t.Mcp.File))
	entry, err := ReadClaudeEntry(file)
	if err != nil || !entry.Found {
		return step, false, false // unreadable or absent: setup added nothing there
	}
	if !entry.Valid || !McpSameArgs(entry.args, McpServeArgs) || !entry.managed {
		if o.Out != nil {
			fmt.Fprintf(o.Out, "%s: the MCP server %q in %s was not registered by setup, left as it is\n\n",
				t.Name, McpServerName, Display(o.Home, file))
		}
		return step, false, false
	}
	cli, have := claudeCLI(o)
	if !have {
		if o.Out != nil {
			fmt.Fprintf(o.Out, "%s: the MCP server %q is registered, but `claude` is not on PATH; remove it with `claude mcp remove -s user %s`\n\n",
				t.Name, McpServerName, McpServerName)
		}
		return step, false, false
	}
	args := []string{"mcp", "remove", "-s", "user", McpServerName}
	return mcpClaudeStep{target: t, claude: cli, args: args, shown: "claude " + mcpShown(args...)}, true, false
}

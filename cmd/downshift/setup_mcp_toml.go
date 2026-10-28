// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Markers around the block setup owns inside ~/.codex/config.toml. Nothing
// outside them is ever touched; the file is edited as text so the user's
// comments and key order survive (a TOML library would drop them).
const (
	mcpBlockBegin = "# downshift:mcp:begin"
	mcpBlockEnd   = "# downshift:mcp:end"
)

var (
	// mcpTableOutside matches a [mcp_servers.downshift] table (or a sub-table).
	mcpTableOutside = regexp.MustCompile(`^[ \t]*\[[ \t]*mcp_servers[ \t]*\.[ \t]*["']?` + mcpServerName + `["']?[ \t]*(\.[^\]]*)?\]`)
	// mcpDottedOutside matches a root-level dotted key or inline table:
	// mcp_servers.downshift.command = ..., mcp_servers.downshift = {...},
	// mcp_servers = { downshift = ... }.
	mcpDottedOutside = regexp.MustCompile(`^[ \t]*mcp_servers[ \t]*(\.[ \t]*["']?` + mcpServerName + `["']?[ \t]*[.=]|=.*["']?\b` + mcpServerName + `\b["']?[ \t]*=)`)
	// mcpKeyInTable matches the key `downshift` inside a [mcp_servers] table.
	mcpKeyInTable  = regexp.MustCompile(`^[ \t]*["']?` + mcpServerName + `["']?[ \t]*[.=]`)
	mcpTableHeader = regexp.MustCompile(`^[ \t]*\[+[ \t]*([^\]]*?)[ \t]*\]+`)
)

// mcpDefinedOutside reports whether text (the file without our block) already
// defines the server downshift in any TOML spelling: a table header, a dotted
// key, an inline table, or a key inside [mcp_servers]. A second definition
// would be a duplicate key and break Codex's config.
func mcpDefinedOutside(text string) bool {
	inServers := false
	for _, line := range strings.Split(text, "\n") {
		if t := strings.TrimSpace(line); t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		if m := mcpTableHeader.FindStringSubmatch(line); m != nil {
			if mcpTableOutside.MatchString(line) {
				return true
			}
			inServers = strings.ReplaceAll(m[1], " ", "") == "mcp_servers"
			continue
		}
		if mcpDottedOutside.MatchString(line) || (inServers && mcpKeyInTable.MatchString(line)) {
			return true
		}
	}
	return false
}

// mcpBlockText returns the managed block of text ("" when there is none).
func mcpBlockText(text string) string {
	start, stop, found, _ := mcpBlockBounds(text)
	if !found {
		return ""
	}
	return text[start:stop]
}

// mcpBlockDiff is the diff shown for config.toml: only the lines inside the
// markers, never their neighbours, which may hold secrets (env = { K = "..." }).
func mcpBlockDiff(shown string, existed bool, before, after string) string {
	return unifiedDiff(diffName(shown, existed && mcpBlockText(before) != "", "a"), diffName(shown, mcpBlockText(after) != "", "b"),
		mcpBlockText(before), mcpBlockText(after))
}

// tomlQuote renders s as a TOML basic string (Windows paths hold backslashes).
func tomlQuote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '\\':
			b.WriteString(`\\`)
		case r == '"':
			b.WriteString(`\"`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\t':
			b.WriteString(`\t`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\u%04X`, r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// mcpTOMLBody is the table inside the markers.
func mcpTOMLBody(bin string) string {
	args := make([]string, len(mcpServeArgs))
	for i, a := range mcpServeArgs {
		args[i] = tomlQuote(a)
	}
	return "[mcp_servers." + mcpServerName + "]\n" +
		"command = " + tomlQuote(bin) + "\n" +
		"args = [" + strings.Join(args, ", ") + "]"
}

// mcpBlockBounds locates the managed block: start is the first byte of the
// begin marker, stop the byte after the end marker's line. found is false when
// there is no block; a half block is an error.
func mcpBlockBounds(text string) (start, stop int, found bool, err error) {
	start = strings.Index(text, mcpBlockBegin)
	end := strings.Index(text, mcpBlockEnd)
	switch {
	case start < 0 && end < 0:
		return 0, 0, false, nil
	case start < 0 || end < start:
		return 0, 0, false, errors.New("the " + mcpBlockBegin + " / " + mcpBlockEnd + " markers are unbalanced; fix or remove them and rerun setup")
	}
	stop = end + len(mcpBlockEnd)
	if strings.HasPrefix(text[stop:], "\n") {
		stop++
	}
	return start, stop, true, nil
}

// removeMcpBlock returns text without the managed block and the blank line
// mergeMarkedBlock put before it.
func removeMcpBlock(text string) string {
	start, stop, found, _ := mcpBlockBounds(text)
	if !found {
		return text
	}
	head := text[:start]
	if strings.HasSuffix(head, "\n\n") {
		head = head[:len(head)-1]
	}
	return head + text[stop:]
}

// planCodexMcp plans the config.toml edit. ok is false when nothing is
// planned, which includes a conflict or an unreadable file: those are printed
// as warnings and left as they are.
func planCodexMcp(o setupOptions, t setupTarget) (p setupPlan, ok bool) {
	path := filepath.Join(o.Home, filepath.FromSlash(t.Mcp.File))
	shown := setupDisplay(o.Home, path)
	p = setupPlan{target: t, path: path}
	raw, err := os.ReadFile(path)
	switch {
	case err == nil:
		p.existed, p.before = true, raw
	case !errors.Is(err, os.ErrNotExist):
		fmt.Fprintf(o.Err, "warning: %s: %s: %v\n  the MCP server was not registered; the file was left as it is\n\n", t.Name, shown, err)
		return p, false
	}
	text := string(raw)
	if _, _, _, err := mcpBlockBounds(text); err != nil {
		fmt.Fprintf(o.Err, "warning: %s: %s: %v\n  the MCP server was not registered; the file was left as it is\n\n", t.Name, shown, err)
		return p, false
	}
	if mcpDefinedOutside(removeMcpBlock(text)) {
		fmt.Fprintf(o.Err, "warning: %s: %s already has a [mcp_servers.%s] table outside the downshift markers; "+
			"it was left as it is. Delete that table and rerun setup to let Downshift manage it\n\n",
			t.Name, shown, mcpServerName)
		return p, false
	}
	p.after = mergeMarkedBlock(raw, mcpBlockBegin, mcpBlockEnd, mcpTOMLBody(o.Binary))
	if !p.changed() {
		fmt.Fprintf(o.Out, "%s: MCP server %q already up to date (%s)\n\n", t.Name, mcpServerName, shown)
		return p, false
	}
	verb := "will change"
	if o.DryRun {
		verb = "would change"
	}
	fmt.Fprintf(o.Out, "%s: %s %s (only the downshift block; the rest of the file is not shown or touched)\n", t.Name, shown, verb)
	fmt.Fprint(o.Out, mcpBlockDiff(shown, p.existed, string(p.before), string(p.after)))
	fmt.Fprintln(o.Out)
	return p, true
}

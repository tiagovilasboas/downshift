// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package docscheck_test

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/clispec"
)

const cliDoc = "docs/reference/cli.md"

var (
	headingLine = regexp.MustCompile("^#{2,4} ")
	codeSpan    = regexp.MustCompile("`([^`]+)`")
	longFlag    = regexp.MustCompile(`(?:^|[\s` + "`" + `\[|(,])(--[a-z][a-z0-9-]*)`)
)

// globalFlags are accepted by every command before it parses its own flags.
var globalFlags = map[string]bool{"--help": true, "--plain": true, "--version": true}

// cliIndex resolves the words after "downshift" to a registry command.
type cliIndex struct {
	byName map[string]clispec.Command // canonical commands, aliases resolved
}

func newCLIIndex(cmds []clispec.Command) cliIndex {
	idx := cliIndex{byName: map[string]clispec.Command{}}
	for _, c := range cmds {
		idx.byName[c.Name] = c
	}
	for _, c := range cmds {
		if c.AliasOf != "" {
			if target, ok := idx.byName[c.AliasOf]; ok {
				idx.byName[c.Name] = target
			}
		}
	}
	return idx
}

// resolve returns the longest registry command that is a word prefix of words.
func (x cliIndex) resolve(words []string) (clispec.Command, bool) {
	for n := len(words); n > 0; n-- {
		if c, ok := x.byName[strings.Join(words[:n], " ")]; ok {
			return c, true
		}
	}
	return clispec.Command{}, false
}

// downshiftRefs returns the commands named as "downshift <words>" in the
// backtick spans of a line, or in the whole line when it has no span.
func (x cliIndex) downshiftRefs(text string) []clispec.Command {
	var out []clispec.Command
	spans := codeSpan.FindAllStringSubmatch(text, -1)
	candidates := []string{text}
	if len(spans) > 0 {
		candidates = nil
		for _, s := range spans {
			candidates = append(candidates, s[1])
		}
	}
	for _, c := range candidates {
		i := strings.Index(c, "downshift ")
		if i < 0 {
			continue
		}
		var words []string
		for _, w := range strings.Fields(c[i+len("downshift "):]) {
			if strings.HasPrefix(w, "-") || strings.ContainsAny(w, "<[|=") {
				break
			}
			words = append(words, w)
		}
		if cmd, ok := x.resolve(words); ok {
			out = append(out, cmd)
		}
	}
	return out
}

// headingCommands lists the commands the headings of cli.md name.
func (x cliIndex) headingCommands(lines []docLine) map[string]bool {
	seen := map[string]bool{}
	for _, l := range lines {
		if l.File == cliDoc && headingLine.MatchString(l.Text) {
			for _, c := range x.downshiftRefs(l.Text) {
				seen[c.Name] = true
			}
		}
	}
	return seen
}

// uncoveredCommands lists the documentable commands no heading names.
func uncoveredCommands(lines []docLine, cmds []clispec.Command) []string {
	idx := newCLIIndex(cmds)
	seen := idx.headingCommands(lines)
	var missing []string
	for _, c := range cmds {
		if c.Hidden || c.AliasOf != "" || c.Legacy || seen[c.Name] {
			continue
		}
		missing = append(missing, c.Name)
	}
	return missing
}

// badFlags lists every --flag in cli.md that its command does not have, and
// every deprecated flag outside a "deprecated" paragraph.
func badFlags(lines []docLine, cmds []clispec.Command) []string {
	idx := newCLIIndex(cmds)
	var bad []string
	var section []clispec.Command
	deprecated := false // the current paragraph says "deprecated"
	for _, l := range lines {
		if l.File != cliDoc {
			continue
		}
		if strings.TrimSpace(l.Text) == "" || headingLine.MatchString(l.Text) {
			deprecated = false
		}
		if headingLine.MatchString(l.Text) {
			section = idx.downshiftRefs(l.Text)
			continue
		}
		deprecated = deprecated || strings.Contains(strings.ToLower(l.Text), "deprecated")
		text := l.Text
		if i := strings.Index(text, " -- "); i >= 0 {
			text = text[:i] // what follows runs another program
		}
		scope := append(idx.downshiftRefs(text), section...)
		if len(scope) == 0 {
			continue
		}
		for _, m := range longFlag.FindAllStringSubmatch(text, -1) {
			if globalFlags[m[1]] {
				continue
			}
			if state := flagState(scope, m[1]); state == "missing" {
				bad = append(bad, l.File+":"+strconv.Itoa(l.N)+" "+m[1]+" is not a flag of "+scope[0].Name)
			} else if state == "deprecated" && !deprecated {
				bad = append(bad, l.File+":"+strconv.Itoa(l.N)+" "+m[1]+" is deprecated; say so on the same paragraph")
			}
		}
	}
	return bad
}

// flagState is "ok", "deprecated" or "missing" across the commands in scope.
func flagState(scope []clispec.Command, flag string) string {
	state := "missing"
	for _, c := range scope {
		for _, f := range c.Flags {
			if f.Name == flag && !f.Deprecated {
				return "ok"
			}
			if f.Name == flag {
				state = "deprecated"
			}
		}
	}
	return state
}

func TestCLIDocHasASectionPerCommand(t *testing.T) {
	for _, name := range uncoveredCommands(guardedLines(t, repoRoot(t)), clispec.Commands()) {
		t.Errorf("%s has no heading for %q; add a section", cliDoc, name)
	}
}

func TestCLIDocFlagsExistOnTheirCommand(t *testing.T) {
	for _, b := range badFlags(guardedLines(t, repoRoot(t)), clispec.Commands()) {
		t.Error(b)
	}
}

func TestCLIGuardsBite(t *testing.T) {
	cmds := []clispec.Command{
		{Name: "doctor", Flags: []clispec.Flag{{Name: "--json"}}},
		{Name: "setup", Flags: []clispec.Flag{{Name: "--mcp", Deprecated: true}}},
		{Name: "gone"},
	}
	doc := func(texts ...string) []docLine {
		var out []docLine
		for i, s := range texts {
			out = append(out, docLine{cliDoc, i + 1, s})
		}
		return out
	}
	if got := uncoveredCommands(doc("### `downshift doctor`"), cmds); len(got) != 2 {
		t.Errorf("missing sections: got %v", got)
	}
	if got := badFlags(doc("### `downshift doctor`", "- `--json` ok", "- `--bogus` no"), cmds); len(got) != 1 {
		t.Errorf("bogus flag: got %v", got)
	}
	if got := badFlags(doc("### `downshift setup`", "- `--mcp` still works"), cmds); len(got) != 1 {
		t.Errorf("deprecated flag without label: got %v", got)
	}
	if got := badFlags(doc("### `downshift setup`", "- `--mcp` is Deprecated"), cmds); len(got) != 0 {
		t.Errorf("labelled deprecated flag: got %v", got)
	}
}

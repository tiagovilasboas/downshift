// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "github.com/tiagovilasboas/downshift-ai/internal/compressor"

// commandPlan names the compressor format of a command-directed family and
// what the compressor needs to know about the argv (compressor.CommandHint).
// Most families have one format, in commandFormats; the file-read family and
// the rg and grep match families choose by their flags.
func commandPlan(family Family, argv []string) (compressor.Format, compressor.CommandHint) {
	if len(argv) == 0 {
		return commandFormats[family], compressor.CommandHint{}
	}
	switch family {
	case FamilyFileRead:
		return compressor.FormatFileRead, fileReadHint(argv)
	case FamilyCurl, FamilyWget:
		return compressor.FormatHTTPFetch, fetchHint(argv)
	case FamilyRg:
		return rgPlan(argv[1:])
	case FamilyGrep:
		return grepPlan(argv[1:])
	}
	return commandFormats[family], compressor.CommandHint{}
}

// rgPlan: -l is a list of paths, -c is path:count, -n is path:line:text, and
// without any of them rows are path:text under a path operand.
func rgPlan(args []string) (compressor.Format, compressor.CommandHint) {
	seen, operands, ok := rgParse(args, rgMatchFlags)
	if !ok {
		return compressor.FormatGrep, compressor.CommandHint{}
	}
	if !(seen["-e"] || seen["--regexp"]) && len(operands) > 0 {
		operands = operands[1:]
	}
	return searchFormat(rgListing(seen), rgCounting(seen), seen["-n"] || seen["--line-number"]), compressor.CommandHint{Roots: operands}
}

// grepPlan reads the same choice from grep's flags.
func grepPlan(args []string) (compressor.Format, compressor.CommandHint) {
	g, ok := parseGrep(args)
	if !ok {
		return compressor.FormatGrep, compressor.CommandHint{}
	}
	rootless := len(g.operands) == 0 && g.has("rR") // grep -r with no path searches the working directory
	roots := g.operands
	if rootless {
		roots = []string{"."}
	}
	return searchFormat(g.has("l"), g.has("c"), g.has("n")), compressor.CommandHint{Roots: roots}
}

func searchFormat(listing, counting, numbered bool) compressor.Format {
	switch {
	case listing:
		return compressor.FormatFind
	case counting:
		return compressor.FormatGrepCounts
	case numbered:
		return compressor.FormatGrep
	}
	return compressor.FormatGrepRows
}

// fileReadHint tells the compressor which file was read (shell-quoted when it
// needs it, for the follow-up command in the note), the file line number of the
// first output line, and whether the output is a tail.
func fileReadHint(argv []string) compressor.CommandHint {
	h := compressor.CommandHint{First: 1}
	switch argv[0] {
	case "cat":
		h.Name = displayWord(argv[1])
	case "head":
		_, file, _ := headTailArgs(argv[1:])
		h.Name = displayWord(file)
	case "tail":
		_, file, _ := headTailArgs(argv[1:])
		h.Name, h.Tail = displayWord(file), true
	case "sed":
		from, _, file, _ := sedRangeArgs(argv[1:])
		h.Name, h.First = displayWord(file), from
	case "jq":
		h.Name, h.JQ = displayWord(argv[len(argv)-1]), true
	}
	return h
}

// displayWord is w as it can be pasted into a shell: bare when every byte is
// in the policy's safe set, single-quoted otherwise.
func displayWord(w string) string {
	for i := 0; i < len(w); i++ {
		if w[i] == ' ' || !safeByte(w[i]) {
			return QuoteWord(w)
		}
	}
	return w
}

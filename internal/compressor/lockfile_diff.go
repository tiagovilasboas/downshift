// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/lazyre"
)

// FormatLockfileDiff marks a git diff whose every file is a lockfile
// (package-lock.json, yarn.lock, pnpm-lock.yaml, Cargo.lock). Such a diff is
// thousands of lines of integrity hashes and resolved URLs; the agent needs
// which packages moved.
const FormatLockfileDiff Format = "lockfile_diff"

// lockfileListCap bounds the package lines of a summary.
const lockfileListCap = 100

var reGitDiffFile = lazyre.New(`^diff --git a/(.+) b/(.+)$`)

// diffStructure lists the line starts a unified diff of a text file may have
// outside hunks. Anything else in a lockfile diff is not understood.
var diffStructure = []string{
	"index ", "--- ", "+++ ", "new file mode", "deleted file mode", "old mode",
	"new mode", "similarity index", "rename from", "rename to",
}

// isLockfileDiff reports whether input is a bare unified diff that touches
// lockfiles only.
func isLockfileDiff(input []byte) bool {
	if !isGitDiff(input) {
		return false
	}
	files := 0
	for _, line := range strings.Split(string(input), "\n") {
		if m := reGitDiffFile.FindStringSubmatch(line); m != nil {
			if !isLockfilePath(m[2]) {
				return false
			}
			files++
		}
	}
	return files > 0
}

// lockSection is one file of a lockfile diff.
type lockSection struct {
	file  string
	lines []string // hunk lines, "@@" headers included
}

// splitLockSections cuts a lockfile diff into its files. It reports false when
// a line is neither diff structure nor a hunk line, in which case the diff is
// not one this compressor understands.
func splitLockSections(input []byte) ([]lockSection, bool) {
	var out []lockSection
	inHunk := false
	for _, line := range strings.Split(strings.TrimSuffix(string(input), "\n"), "\n") {
		if m := reGitDiffFile.FindStringSubmatch(line); m != nil {
			out = append(out, lockSection{file: m[2]})
			inHunk = false
			continue
		}
		if len(out) == 0 {
			return nil, false
		}
		cur := &out[len(out)-1]
		if strings.HasPrefix(line, "@@") {
			inHunk = true
			cur.lines = append(cur.lines, line)
			continue
		}
		if inHunk && (line == "" || strings.IndexByte(" +-\\", line[0]) >= 0) {
			cur.lines = append(cur.lines, line)
			continue
		}
		if inHunk || !hasAnyPrefix(line, diffStructure) {
			return nil, false
		}
	}
	return out, len(out) > 0
}

func hasAnyPrefix(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// compressLockfileDiff summarises a lockfile diff to the packages added,
// removed and changed. Anything it does not understand, any danger word or
// secret outside the package lines, and a diff with no package movement go
// through the ordinary git diff compressor, which keeps every change line.
func compressLockfileDiff(input []byte) ([]byte, bool, string) {
	secs, ok := splitLockSections(input)
	if !ok {
		return compressGitDiff(input)
	}
	var b strings.Builder
	listed, moved := 0, 0
	for _, sec := range secs {
		scan := scanLockfile(path.Base(sec.file), sec.lines)
		if scan.unsafe {
			return compressGitDiff(input)
		}
		sum := summariseLock(scan)
		moved += sum.total()
		b.WriteString(sum.render(sec.file, &listed))
	}
	if moved == 0 {
		return compressGitDiff(input)
	}
	out := fmt.Sprintf("Lockfile diff, %d file(s): package changes only, hunks cut "+
		"(run git diff -- <file> for the lines)\n%s", len(secs), b.String())
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return []byte(out), true, fmt.Sprintf("summarised lockfile diff (%d files, %d packages)", len(secs), moved)
}

// lockSummary is the package movement of one lockfile.
type lockSummary struct {
	added, removed map[string][]string // name -> versions
	changed        map[string][2][]string
	unattributed   int
}

func (l lockSummary) total() int {
	return len(l.added) + len(l.removed) + len(l.changed) + l.unattributed
}

// summariseLock classifies the events of one file. A version present on both
// sides of a package is unchanged and dropped.
func summariseLock(scan lockScan) lockSummary {
	adds, rems := map[string]map[string]bool{}, map[string]map[string]bool{}
	for _, e := range scan.events {
		side := adds
		if e.sign == '-' {
			side = rems
		}
		if side[e.name] == nil {
			side[e.name] = map[string]bool{}
		}
		side[e.name][e.version] = true
	}
	sum := lockSummary{added: map[string][]string{}, removed: map[string][]string{},
		changed: map[string][2][]string{}, unattributed: scan.unattributed}
	names := map[string]bool{}
	for n, vs := range adds {
		for v := range vs {
			if rems[n][v] {
				delete(vs, v)
				delete(rems[n], v)
			}
		}
		names[n] = true
	}
	for n := range rems {
		names[n] = true
	}
	for n := range names {
		a, r := sortedKeys(adds[n]), sortedKeys(rems[n])
		switch {
		case len(a) > 0 && len(r) > 0:
			sum.changed[n] = [2][]string{r, a}
		case len(a) > 0:
			sum.added[n] = a
		case len(r) > 0:
			sum.removed[n] = r
		}
	}
	return sum
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// render prints one file: its counts and, until the shared cap, the packages.
func (l lockSummary) render(file string, listed *int) string {
	if l.total() == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %d added, %d removed, %d changed", file, len(l.added), len(l.removed), len(l.changed))
	if l.unattributed > 0 {
		fmt.Fprintf(&b, ", %d version lines of unknown package", l.unattributed)
	}
	b.WriteString("\n")
	emit := func(mark string, name, detail string) {
		if *listed >= lockfileListCap {
			return
		}
		*listed++
		fmt.Fprintf(&b, "  %s %s %s\n", mark, name, detail)
	}
	for _, n := range sortedMapKeys(l.added) {
		emit("+", n, strings.Join(l.added[n], ","))
	}
	for _, n := range sortedMapKeys(l.removed) {
		emit("-", n, strings.Join(l.removed[n], ","))
	}
	changedNames := make([]string, 0, len(l.changed))
	for n := range l.changed {
		changedNames = append(changedNames, n)
	}
	sort.Strings(changedNames)
	for _, n := range changedNames {
		v := l.changed[n]
		emit("~", n, strings.Join(v[0], ",")+" -> "+strings.Join(v[1], ","))
	}
	if *listed >= lockfileListCap {
		b.WriteString("  ... list capped at 100 packages\n")
	}
	return b.String()
}

func sortedMapKeys(m map[string][]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

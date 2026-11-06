// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"regexp"
)

// Stack-trace fold for the generic chain. A run of more than stackFoldMin
// contiguous frames of one language keeps its first and last stackKeep frames
// and the middle becomes "[N frames elided]". Only the frame lines of such a
// run leave the danger gate; every other line, including the message above
// the run, is still gated, so a trace headed by "Error: boom" stays raw. A
// short run, an unrecognised "at x:N" line or a mixed run is not folded and
// keeps tripping the gate as before.
const (
	stackFoldMin = 12
	stackKeep    = 5
)

// stackKind names the language a frame belongs to; a run never mixes kinds.
type stackKind int

const (
	stackNone stackKind = iota
	stackJS
	stackJava
	stackPython
	stackGo
)

var (
	reStackJS     = regexp.MustCompile(`^\s+at (?:async )?(?:[\w$.<>\[\]]+(?: \[as [\w$]+\])? \(.+:\d+:\d+\)|[^\s()]+:\d+:\d+)$`)
	reStackJava   = regexp.MustCompile(`^\s+at [\w$.<>/]+\([^()\s][^()]*\)$`)
	reStackPyFile = regexp.MustCompile(`^  File ".+", line \d+(?:, in .+)?$`)
	reStackPyCode = regexp.MustCompile(`^    \S`)
	reStackGoFn   = regexp.MustCompile(`^[A-Za-z_]\S*\(.*\)$`)
	reStackGoFile = regexp.MustCompile(`^\t\S+\.go:\d+(?: \+0x[0-9a-f]+)?$`)
)

// stackRun is a foldable run: the index of the first line of each frame and
// the index after the last line of the run.
type stackRun struct {
	starts []int
	end    int
}

// frameAt returns the kind and the number of lines of the frame starting at
// lines[i], or stackNone. Lines over maxRegexLine are never frames.
func frameAt(lines []string, i int) (stackKind, int) {
	l := lines[i]
	if len(l) > maxRegexLine {
		return stackNone, 0
	}
	next := ""
	if i+1 < len(lines) && len(lines[i+1]) <= maxRegexLine {
		next = lines[i+1]
	}
	switch {
	case reStackJS.MatchString(l):
		return stackJS, 1
	case reStackJava.MatchString(l):
		return stackJava, 1
	case reStackPyFile.MatchString(l):
		if reStackPyCode.MatchString(next) {
			return stackPython, 2
		}
		return stackPython, 1
	case reStackGoFn.MatchString(l) && reStackGoFile.MatchString(next):
		return stackGo, 2
	}
	return stackNone, 0
}

// findStackRuns returns every run longer than stackFoldMin frames.
func findStackRuns(lines []string) []stackRun {
	var runs []stackRun
	for i := 0; i < len(lines); {
		kind, n := frameAt(lines, i)
		if kind == stackNone {
			i++
			continue
		}
		run := stackRun{}
		for {
			run.starts = append(run.starts, i)
			i += n
			if i >= len(lines) {
				break
			}
			var k stackKind
			if k, n = frameAt(lines, i); k != kind {
				break
			}
		}
		run.end = i
		if len(run.starts) > stackFoldMin {
			runs = append(runs, run)
		}
	}
	return runs
}

// foldStackRuns elides the middle of each run. It returns the lines the
// danger gate must read (everything outside the frames of a run), the folded
// lines, and whether it folded anything. With no run both are the input.
func foldStackRuns(lines []string) (gated, folded []string, ok bool) {
	runs := findStackRuns(lines)
	if len(runs) == 0 {
		return lines, lines, false
	}
	pos := 0
	for _, r := range runs {
		first := r.starts[0]
		headEnd := r.starts[stackKeep]
		tailStart := r.starts[len(r.starts)-stackKeep]
		gated = append(gated, lines[pos:first]...)
		folded = append(folded, lines[pos:headEnd]...)
		folded = append(folded, fmt.Sprintf("[%d frames elided]", len(r.starts)-2*stackKeep))
		folded = append(folded, lines[tailStart:r.end]...)
		pos = r.end
	}
	gated = append(gated, lines[pos:]...)
	folded = append(folded, lines[pos:]...)
	return gated, folded, true
}

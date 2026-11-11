// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"bytes"
	"fmt"
	"strings"
	"unicode/utf8"
)

// embeddedBlock reads one complete passing test-runner block that starts at
// line i of the scan. It returns the runner's own pass summary and the index
// just past the block, or ok=false when line i opens no block or the block
// cannot be bounded and parsed with certainty. On ok=false, next is where the
// scan resumes: lines i to next stay verbatim (next is i+1 when line i opens
// no block). A refused block is skipped whole, since any later opener inside
// it reaches the same end; with the precomputed closers this keeps the fold
// linear in the number of lines.
type embeddedBlock func(s *blockScan, i int) (summary string, next int, ok bool)

// embeddedBlocks lists the runner shapes that have a certain start line and a
// certain end line inside another program's transcript. jest has neither (its
// run opens with a PASS file row like any other and console output interleaves
// freely), and pytest -q prints a bare summary with no header, so both stay out.
var embeddedBlocks = []embeddedBlock{goVerboseBlock, pytestBlock, vitestBlock}

// FoldEmbeddedRuns replaces each complete passing test-runner block inside
// another program's output (make, an npm script) with that runner's existing
// pass summary; every other line is kept verbatim and in order. It is for a
// zero exit only. The output stays raw when it carries a failure of any of the
// runners, when no block folds, or when the result is not shorter.
func FoldEmbeddedRuns(input []byte) Result {
	res := Result{Output: input, Format: FormatGoTestRuns, OriginalBytes: len(input), ReducedBytes: len(input)}
	switch {
	case len(input) == 0:
		res.Reason = "empty_input"
		return res
	case len(input) > MaxProcessableBytes:
		res.Reason = "input_exceeds_memory_bound"
		return res
	case bytes.IndexByte(input, 0) != -1 || !utf8.Valid(input):
		res.Reason = "binary_or_invalid_utf8"
		return res
	}
	text, reason := screenText(input)
	if reason != "" {
		res.Reason = reason
		return res
	}
	if reGoTestFailure.Match(text) || rePytestFailSummary.Match(text) || reVitestFail.Match(text) {
		res.Reason = "embedded_runs_failure_preserved"
		return res
	}
	lines := strings.Split(strings.TrimSuffix(string(text), "\n"), "\n")
	scan := newBlockScan(lines)
	var out strings.Builder
	folded := 0
	for i := 0; i < len(lines); {
		summary, next, ok := foldBlockAt(scan, i)
		if ok {
			out.WriteString(summary)
			folded++
			i = next
			continue
		}
		for ; i < next; i++ {
			out.WriteString(lines[i])
			out.WriteByte('\n')
		}
	}
	if folded == 0 {
		res.Reason = "embedded_runs_none_folded"
		return res
	}
	if out.Len() >= len(input) {
		res.Reason = "no_compression_gain"
		return res
	}
	res.Applied = true
	res.Output = []byte(out.String())
	res.ReducedBytes = out.Len()
	res.Reason = fmt.Sprintf("folded %d embedded test runs", folded)
	return res
}

// FoldTestRuns folds what another program's transcript embeds: complete
// passing runner blocks first, then runs of clean go test package rows. It
// returns the first Result that applies, or the plain-rows refusal.
func FoldTestRuns(input []byte) Result {
	emb := FoldEmbeddedRuns(input)
	if !emb.Applied {
		return FoldGoTestRuns(input)
	}
	rows := FoldGoTestRuns(emb.Output)
	if !rows.Applied {
		return emb
	}
	rows.OriginalBytes = len(input)
	rows.Reason = emb.Reason + "; " + rows.Reason
	return rows
}

// foldBlockAt tries each runner at line i. On a refusal it returns the furthest
// resume index a runner gave, at least i+1.
func foldBlockAt(s *blockScan, i int) (string, int, bool) {
	resume := i + 1
	for _, b := range embeddedBlocks {
		summary, next, ok := b(s, i)
		if ok {
			return summary, next, true
		}
		resume = max(resume, next)
	}
	return "", resume, false
}

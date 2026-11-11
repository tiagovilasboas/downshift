// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"strings"
)

// held is what a finished run prints: the compacted text, or the raw output
// with the reason it stayed raw.
type held struct {
	out                 []byte
	compacted, folded   bool
	passthrough, detail string
}

// compactHeld is the one compaction of a finished run's output, shared by the
// wrapper (gate.finish) and by output captured after a run (CompactCaptured):
// the family's compressor, the CCR fold of a clean run it kept raw, then the
// balanced cut. keep is the storage port for this run (nil: nothing stored, so
// nothing is cut with loss). When compacted is false, out is data.
func compactHeld(argv []string, data []byte, family Family, exitCode int, wroteStderr bool, keep func([]byte) (string, bool), desktop func([]byte) []byte, balanced bool, bal balanceConfig) held {
	var h held
	h.out, h.passthrough, h.detail = compactOutputFor(argv, exitCode, data, family, wroteStderr)
	h.compacted = h.passthrough == ""
	if h.compacted {
		h.out = keepRawFor(family, exitCode, keep, data, h.out)
		h.out = applyDesktop(desktop, family, data, h.out)
	} else if folded, ok := keepUncompacted(keep, data, exitCode, family, wroteStderr); ok {
		h = held{out: folded, compacted: true, folded: true}
	}
	if balanced {
		shown := data
		if h.compacted {
			shown = h.out
		}
		if cut, ok := balance(keep, family, data, shown, bal); ok {
			h = held{out: cut, compacted: true, folded: true}
		}
	}
	if !h.compacted {
		h.out = data
	}
	return h
}

// Captured is the compaction of one output read after its run ended.
type Captured struct {
	Output              []byte // what to show: the compacted text, or the raw output
	Compacted, Folded   bool
	Passthrough, Detail string // why the output stayed raw (Passthrough*, Detail*)
	KeptID              string // the CCR id the marker names, when raw output was stored
}

// CompactCaptured compacts data, the output a run of argv already printed,
// with exactly the steps the wrapper applies (compactHeld): the post-run path
// of a harness that can replace a tool result after the run. family is
// FamilyNone when the command is not one plain allowlisted command (a pipe, a
// redirect, an unknown program): only the balanced log cut can then apply,
// and only once keep stored the raw output. A family whose output the wrapper
// would not hold (a file read, wc of a path it cannot check from here) and a
// family whose raw output must never be stored keep data whole. A panic
// returns data unchanged. session is the caller's hashed session ("" when
// unknown): with it the marker's retrieve hint names the session (SessionRef).
func CompactCaptured(argv []string, family Family, data []byte, exitCode int, keep func([]byte) (string, bool), balanced bool, session string) (res Captured) {
	res = Captured{Output: data, Passthrough: PassthroughUnrecognized}
	defer func() {
		if recover() != nil {
			res = Captured{Output: data, Passthrough: PassthroughUnrecognized, Detail: DetailCompressorPanic}
		}
	}()
	if len(data) == 0 {
		res.Passthrough = PassthroughSmallOutput
		return res
	}
	if family == FamilyFileRead || family == FamilyWc || family == FamilyAgentDesktop {
		return res
	}
	if rawStoreRefused[family] {
		keep = nil
	}
	var kept string
	h := compactHeld(argv, data, family, exitCode, false, captureKept(keep, &kept, session), nil, balanced, balanceConfig{})
	res = Captured{Output: h.out, Compacted: h.compacted, Folded: h.folded, Passthrough: h.passthrough, Detail: h.detail}
	if h.compacted {
		res.KeptID = kept
	}
	return res
}

// SimpleFamily is the family of a command that is one plain allowlisted
// program: no shell metacharacter, within the policy's length bound, leading
// NAME=value assignments skipped. Any other command (a pipe, a redirect, a
// list, a quote) is FamilyNone, because its output is not that program's
// alone. The argv is returned for the compressors that read the command line.
func SimpleFamily(command string) (Family, []string) {
	trimmed := strings.Trim(command, " ")
	if trimmed == "" || len(trimmed) > maxCommandBytes {
		return FamilyNone, nil
	}
	for i := 0; i < len(trimmed); i++ {
		if !safeByte(trimmed[i]) {
			return FamilyNone, nil
		}
	}
	_, argv := splitEnvAssignments(strings.Fields(trimmed))
	if len(argv) == 0 || argv[0] == "downshift" {
		return FamilyNone, nil
	}
	return FamilyOf(argv), argv
}

// IsWrapped reports whether command is already a `downshift compact` call (the
// shell rewrite's form, a direct call, or a chain with one wrapped segment).
// Its output went through the wrapper, so nothing should compact it again.
func IsWrapped(command string) bool {
	if strings.Contains(command, execTail) || strings.Contains(command, " compact --session=") || strings.Contains(command, " compact --harness=") {
		return true
	}
	words := strings.Fields(command)
	return len(words) > 1 && strings.HasSuffix(words[0], "downshift") && (words[1] == "compact" || words[1] == "exec" || words[1] == "compactor")
}

// HasMarker reports whether text ends with a line the wrapper writes last (its
// marker or one of its tail notes), so it already went through the wrapper. A
// marker quoted anywhere else (a diff of this repository, a grep hit) does not
// count.
func HasMarker(text string) bool {
	text = strings.TrimRight(text, "\n")
	return strings.HasPrefix(text[strings.LastIndexByte(text, '\n')+1:], markerPrefix)
}

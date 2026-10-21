// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "strings"

// Chains: a list of commands joined by `;` or `&&`, the form agents use to
// run a formatter or a diff before the tests.
//
//	gofmt -l x; go test ./pkg
//	git diff && go test ./... 2>&1 | tail -15
//
// Each segment is decided alone, by the simple policy or a compound shape;
// only allowlisted segments are wrapped, every other segment and every
// separator stays byte-identical. The wrapper keeps each segment's exit code,
// so `;` still returns the last segment's code and `&&` still short-circuits.
//
// One segment the policy cannot prove safe (a quote, `||`, a lone `&`, a
// redirect, an expansion, a pipe that is not `| tail/head -N`) refuses the
// whole command: no segment is wrapped.

// ReasonAllowlistedChain is the reason for a wrapped chain.
const ReasonAllowlistedChain = "allowlisted_chain"

// wrappedMarks find an already wrapped command, bare or with a quoted path.
var wrappedMarks = []string{"downshift compact", "downshift' compact"}

// decideChain wraps the allowlisted segments of a `;`/`&&` chain, or refuses
// the whole command.
func decideChain(trimmed, exe string) Decision {
	for _, m := range wrappedMarks {
		if strings.Contains(trimmed, m) {
			return Decision{Reason: ReasonAlreadyWrapped}
		}
	}
	segs, seps, ok := splitChain(trimmed)
	if !ok {
		return Decision{Reason: ReasonUnsafeChar}
	}
	var out strings.Builder
	var first *Decision
	var others []string
	for i, seg := range segs {
		if i > 0 {
			out.WriteString(seps[i-1])
		}
		d, keep := decideSegment(seg, exe)
		if !keep && !d.Rewrite {
			return Decision{Family: d.Family, Reason: d.Reason}
		}
		if !d.Rewrite {
			out.WriteString(seg)
			others = append(others, strings.Join(strings.Fields(seg), " "))
			continue
		}
		out.WriteString(d.Command)
		others = append(others, d.Segments...)
		if first == nil {
			d := d
			first = &d
			continue
		}
		others = append(others, d.Argv)
	}
	if first == nil {
		return Decision{Reason: ReasonNotAllowlisted}
	}
	return Decision{
		Rewrite:  true,
		Command:  out.String(),
		Family:   first.Family,
		Argv:     first.Argv,
		Segments: others,
		Reason:   ReasonAllowlistedChain,
	}
}

// decideSegment decides one chain segment. keep is true when a segment that
// is not wrapped is still provably safe to leave as written: only safe bytes,
// or a compound shape around a safe command.
func decideSegment(seg, exe string) (d Decision, keep bool) {
	d = decideSimple(seg, exe)
	if d.Reason == ReasonUnsafeChar {
		d = decideCompound(seg, exe)
	}
	return d, d.Rewrite || d.Reason == ReasonNotAllowlisted
}

// splitChain cuts a command at each `;` and `&&`. seps[i] is the exact text
// between segs[i] and segs[i+1], spaces included. ok is false with fewer than
// two segments or an empty one.
func splitChain(command string) (segs, seps []string, ok bool) {
	start := 0
	for i := 0; i < len(command); i++ {
		n := 0
		switch {
		case command[i] == ';':
			n = 1
		case command[i] == '&' && i+1 < len(command) && command[i+1] == '&':
			n = 2
		default:
			continue
		}
		seg := strings.TrimRight(command[start:i], " ")
		next := i + n
		for next < len(command) && command[next] == ' ' {
			next++
		}
		if strings.TrimSpace(seg) == "" {
			return nil, nil, false
		}
		segs = append(segs, seg)
		seps = append(seps, command[start+len(seg):next])
		start = next
		i = next - 1
	}
	last := command[start:]
	if len(segs) == 0 || strings.TrimSpace(last) == "" {
		return nil, nil, false
	}
	return append(segs, last), seps, true
}

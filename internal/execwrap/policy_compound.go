// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "strings"

// Compound shapes: the few shell forms agents wrap around an allowlisted
// command most often. Each is matched exactly, word by word; anything else
// keeps the plain policy's answer (unsafe_character).
//
//	cd <dir> && <allowlisted>
//	<allowlisted> | tail -N        (also head, -n N, -nN)
//	<allowlisted> 2>&1 | tail -N
//	cd <dir> && <allowlisted> 2>&1 | tail -N
//	<allowlisted> 2>&1             (a trailing stderr merge, no pipe)
//	cd <dir> && <allowlisted> 2>&1
//
// The 2>&1 merge is kept as written: the wrapper already captures both
// streams of the command, so the rewritten command keeps the same merge on
// the wrapper's own output.
//
// Only the allowlisted command is wrapped; the cd prefix and the pipe suffix
// are kept as written, so the exit code is what it was: cd's on failure, the
// pipe's last command otherwise. <dir> and the pipe's command pass the
// safe-byte gate, so they hold no expansion, quote, glob or tilde.
//
// The wrapped command may also hold quoted words and an exact trailing
// 2>/dev/null, lexed by the quoted policy (policy_quoted.go), as in
// `go test -run 'A|B' ./... 2>&1 | tail -20`. Two trailing forms are refused
// there because the shape around them changes their meaning: `|| true`
// (`a && b || true` applies it to the whole list) and 2>/dev/null before a
// 2>&1 merge (the merge sends stderr back into the pipe). A shape whose
// wrapped command went through the quoted policy carries
// ReasonAllowlistedQuoted, so every guard on the quoted path (a deny or ask
// rule that holds a quote) still applies.

// ReasonAllowlistedCompound is the reason for a wrapped compound shape.
const ReasonAllowlistedCompound = "allowlisted_compound"

const (
	andSep    = " && "
	pipeSep   = " | "
	stderrDup = " 2>&1"
	// maxPipeLines bounds the -N of tail/head: five digits.
	maxPipeLines = 5
)

// decideCompound wraps the allowlisted core of a compound shape, or returns
// the plain policy's unsafe_character answer when the command is no shape.
func decideCompound(trimmed, exe string) Decision {
	rest, prefix, cdSeg := trimmed, "", ""
	if dir, after, ok := cdPrefix(rest); ok {
		cdSeg = "cd " + dir
		prefix = cdSeg + andSep
		rest = after
	}
	rest, suffix, pipeSeg := pipeSuffix(rest)
	if suffix == "" {
		rest, suffix = stderrSuffix(rest)
	}
	if prefix == "" && suffix == "" {
		return Decision{Reason: ReasonUnsafeChar}
	}
	inner := decideSimple(rest, exe)
	if inner.Reason == ReasonUnsafeChar {
		inner = decideQuotedPart(rest, exe, strings.HasPrefix(suffix, stderrDup))
	}
	if !inner.Rewrite {
		return inner
	}
	inner.Command = prefix + inner.Command + suffix
	if inner.Reason != ReasonAllowlistedQuoted {
		inner.Reason = ReasonAllowlistedCompound
	}
	for _, s := range []string{cdSeg, pipeSeg} {
		if s != "" {
			inner.Segments = append(inner.Segments, s)
		}
	}
	return inner
}

// decideQuotedPart is the quoted policy for the command a shape or a chain
// wraps. It answers unsafe_character for anything it does not wrap, so a
// quoted command that is not allowlisted is never kept as written in a chain,
// and it refuses the trailing forms whose meaning the surrounding shape
// changes: `|| true` always, 2>/dev/null when a 2>&1 merge follows.
func decideQuotedPart(part, exe string, mergesStderr bool) Decision {
	if strings.HasSuffix(part, orTrueTail) || (mergesStderr && strings.HasSuffix(part, devNullTail)) {
		return Decision{Reason: ReasonUnsafeChar}
	}
	if d := decideQuoted(part, exe); d.Rewrite {
		return d
	}
	return Decision{Reason: ReasonUnsafeChar}
}

// cdPrefix splits `cd <dir> && rest`. dir is one safe word that is not an
// option.
func cdPrefix(command string) (dir, rest string, ok bool) {
	head, rest, found := strings.Cut(command, andSep)
	if !found {
		return "", "", false
	}
	words := strings.Fields(head)
	if len(words) != 2 || words[0] != "cd" || strings.HasPrefix(words[1], "-") || !allSafe(words[1]) {
		return "", "", false
	}
	return words[1], strings.TrimLeft(rest, " "), true
}

// pipeSuffix splits `core [2>&1] | tail -N`. suffix is what follows the
// wrapped command in the rewrite; seg is the pipe's command alone, for the
// permission check. No shape returns the command unchanged and empty parts.
func pipeSuffix(command string) (core, suffix, seg string) {
	i := strings.LastIndex(command, pipeSep)
	if i < 0 {
		return command, "", ""
	}
	tail := command[i+len(pipeSep):]
	words := strings.Fields(tail)
	if !allSafe(tail) || !lineFilter(words) {
		return command, "", ""
	}
	seg = strings.Join(words, " ")
	core = strings.TrimRight(command[:i], " ")
	redirect := ""
	if strings.HasSuffix(core, stderrDup) {
		redirect = stderrDup
		core = strings.TrimRight(strings.TrimSuffix(core, stderrDup), " ")
	}
	return core, redirect + pipeSep + seg, seg
}

// stderrSuffix splits a trailing ` 2>&1` off a command that has no pipe. The
// token must be exactly that, at the end, so every other redirect (2>/dev/null,
// >file, 1>&2) keeps the command unsafe.
func stderrSuffix(command string) (core, suffix string) {
	if !strings.HasSuffix(command, stderrDup) {
		return command, ""
	}
	return strings.TrimRight(strings.TrimSuffix(command, stderrDup), " "), stderrDup
}

// lineFilter reports whether words are exactly `tail|head -N`, `-n N` or
// `-nN`, with N a short positive count.
func lineFilter(words []string) bool {
	if len(words) < 2 || len(words) > 3 || (words[0] != "tail" && words[0] != "head") {
		return false
	}
	switch {
	case len(words) == 3:
		return words[1] == "-n" && count(words[2])
	case strings.HasPrefix(words[1], "-n"):
		return count(words[1][2:])
	case strings.HasPrefix(words[1], "-"):
		return count(words[1][1:])
	}
	return false
}

func count(s string) bool {
	if s == "" || len(s) > maxPipeLines {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func allSafe(s string) bool {
	for i := 0; i < len(s); i++ {
		if !safeByte(s[i]) {
			return false
		}
	}
	return true
}

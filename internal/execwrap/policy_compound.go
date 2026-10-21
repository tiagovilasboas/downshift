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
//
// Only the allowlisted command is wrapped; the cd prefix and the pipe suffix
// are kept as written, so the exit code is what it was: cd's on failure, the
// pipe's last command otherwise. <dir> passes the same safe-byte gate as the
// command, so it holds no expansion, quote, glob or tilde.

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
	if prefix == "" && suffix == "" {
		return Decision{Reason: ReasonUnsafeChar}
	}
	inner := decideSimple(rest, exe)
	if !inner.Rewrite {
		return inner
	}
	inner.Command = prefix + inner.Command + suffix
	inner.Reason = ReasonAllowlistedCompound
	for _, s := range []string{cdSeg, pipeSeg} {
		if s != "" {
			inner.Segments = append(inner.Segments, s)
		}
	}
	return inner
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

// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"fmt"
	"io"
	"strings"
)

// The runner's side of the two trailing forms the quoted rewrite accepts
// (policy_quoted.go): `2>/dev/null` becomes FlagDiscardStderr and `|| true`
// becomes FlagExitZero, so the wrapper itself keeps what the user asked for
// and still tells the reader what happened.

// ParseTailFlags reads the trailing-form flags at the head of args, before
// the `--`, into o and returns the args that follow them.
func ParseTailFlags(args []string, o *Options) []string {
	for len(args) > 0 {
		switch args[0] {
		case FlagDiscardStderr:
			o.DiscardStderr = true
		case FlagExitZero:
			o.ExitZero = true
		default:
			return args
		}
		args = args[1:]
	}
	return args
}

// ShellExit is the status the wrapper exits with: the program's own code, or
// 0 when the command ended in `|| true`. Result.ExitCode stays the real code.
func ShellExit(o Options, code int) int {
	if o.ExitZero {
		return 0
	}
	return code
}

// tailNotes is the text that ends a compacted output, after its Marker, with
// one line per trailing form that changed what the reader sees: stderr
// discarded, or a real non-zero exit returned as 0.
func tailNotes(o Options, code int) string {
	var b strings.Builder
	if o.DiscardStderr {
		b.WriteString("[downshift compact: stderr discarded (2>/dev/null)]\n")
	}
	if o.ExitZero && code != 0 {
		fmt.Fprintf(&b, "[downshift compact: exit %d, returned 0 (|| true)]\n", code)
	}
	return b.String()
}

// writeTailNotes writes the notes. Run calls it only when the output was
// compacted, so `git rev-parse HEAD 2>/dev/null` still prints just the hash.
func writeTailNotes(w io.Writer, o Options, code int) {
	if w == nil {
		return
	}
	_, _ = io.WriteString(w, tailNotes(o, code))
}

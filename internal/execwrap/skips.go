// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"encoding/json"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift/internal/paths"
)

// SkipsFileName is the log of commands the shell-rewrite hook declined to wrap.
const SkipsFileName = "exec-skips.jsonl"

// Skip reasons: why the hook left a Bash command unwrapped. Each is a fixed
// code, so the log holds no free text. Decide's other reason codes (empty,
// too_long, already_wrapped, ...) are logged as they are.
const (
	// SkipNotAllowlisted: the command is plain but names no allowlisted family.
	SkipNotAllowlisted = "not_allowlisted"
	// SkipMetachar: a pipe, list, quote, redirection or expansion kept the
	// command outside the safe-byte gate.
	SkipMetachar = "metachar"
	// SkipDenied: a user deny or ask rule covers the command.
	SkipDenied = "denied"
	// SkipDisabled: exec is off (config, DOWNSHIFT_EXEC=0) or already nested.
	SkipDisabled = "disabled"
	// SkipAbsPath: the binary is allowlisted but was given by absolute path.
	SkipAbsPath = "abs_path"
)

// SkipEvent is one declined wrap. It holds a reason code and the harness name,
// never the command text, argv, paths or working directory.
type SkipEvent struct {
	TS      string `json:"ts"`
	Reason  string `json:"reason"`
	Harness string `json:"harness,omitempty"`
	// Metachar is the class of the metacharacter behind a metachar skip
	// (semicolon, pipe, ...), and Family the allowlisted family named before
	// it when known. Both optional: older rows and other reasons carry none.
	Metachar string `json:"metachar,omitempty"`
	Family   string `json:"family,omitempty"`
	// Kind is the coarse shape of a metachar skip (heredoc, subst, redirect,
	// ... see MetacharKind). Optional: older rows and other reasons carry none.
	Kind string `json:"kind,omitempty"`
}

// SkipsPath is exec-skips.jsonl in the state directory.
func SkipsPath() (string, error) { return paths.Join(SkipsFileName) }

// SkipReason maps a declined Decision to its skip code. command is read only
// to tell an absolute-path binary from an unknown one; it is not returned or
// stored. It returns "" when the decision wrapped the command.
func SkipReason(d Decision, command string) string {
	if d.Rewrite {
		return ""
	}
	switch d.Reason {
	case "disabled":
		return SkipDisabled
	case "blocked_by_user_rules":
		return SkipDenied
	case ReasonUnsafeChar:
		return SkipMetachar
	case ReasonNotAllowlisted:
		if hasUnsafeByte(command) {
			return SkipMetachar
		}
		if absPathAllowlisted(command) {
			return SkipAbsPath
		}
		return SkipNotAllowlisted
	}
	return d.Reason
}

// hasUnsafeByte reports whether the command holds a shell metacharacter the
// policy still refuses, the case where a chain or compound shape refused it as
// not allowlisted. A quoted word alone is not one (policy_quoted.go).
func hasUnsafeByte(command string) bool { return firstRefused(command) >= 0 }

// absPathAllowlisted reports whether the command's binary is an absolute path
// whose basename would be allowlisted.
func absPathAllowlisted(command string) bool {
	_, argv := splitEnvAssignments(strings.Fields(command))
	if len(argv) == 0 || !strings.HasPrefix(filepath.ToSlash(argv[0]), "/") {
		return false
	}
	argv = append([]string{path.Base(filepath.ToSlash(argv[0]))}, argv[1:]...)
	return FamilyOf(argv) != FamilyNone
}

// AppendSkip writes one record. Callers ignore the error. An empty reason
// writes nothing.
func AppendSkip(p, reason, harness string, now time.Time) error {
	return AppendSkipEvent(p, SkipEvent{Reason: reason, Harness: harness}, now)
}

// AppendSkipEvent writes ev with the timestamp now. An empty reason writes
// nothing.
func AppendSkipEvent(p string, ev SkipEvent, now time.Time) error {
	if ev.Reason == "" {
		return nil
	}
	ev.TS = now.UTC().Format(time.RFC3339)
	line, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	return appendLine(p, line)
}

// SummarizeSkips counts the skip log by reason. A missing file is empty.
func SummarizeSkips(p string) (map[string]int64, error) {
	var out map[string]int64
	err := scanLog(p, func(line []byte) {
		var ev SkipEvent
		if json.Unmarshal(line, &ev) != nil || ev.Reason == "" {
			return
		}
		if out == nil {
			out = map[string]int64{}
		}
		out[ev.Reason]++
	})
	return out, err
}

// ReadSkips reads every record of the skip log at p. A missing file is an
// empty slice. Malformed lines and rows with no reason are skipped.
func ReadSkips(p string) ([]SkipEvent, error) {
	var out []SkipEvent
	err := scanLog(p, func(line []byte) {
		var ev SkipEvent
		if json.Unmarshal(line, &ev) == nil && ev.Reason != "" {
			out = append(out, ev)
		}
	})
	return out, err
}

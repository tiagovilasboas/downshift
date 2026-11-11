// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"bytes"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/tiagovilasboas/downshift-ai/internal/lazyre"
)

// FormatGitRemote is the output of a git operation that runs hooks or talks
// to a remote (git commit, push, pull and fetch): the operation's own lines
// and, around them, the output of the hooks it ran (lefthook, husky, a go test
// run).
const FormatGitRemote Format = "git_remote"

var (
	// Object-transfer progress and the pack statistics printed after it.
	reGitRemoteProgress = lazyre.New(`^(?:(?:Enumerating|Counting|Compressing|Writing|Receiving|Unpacking|Resolving) (?:objects|deltas):.*` +
		`|Delta compression using up to \d+ threads.*` +
		`|Total \d+ \(delta \d+\), reused .*)$`)
	// The pure frame lines of the banner box and the rules a hook runner draws
	// around its steps: a corner followed only by box drawing characters, a
	// rule, or an empty box side. A line with text inside is not a frame.
	reGitRemoteBanner = lazyre.New(`^\s*(?:[╭╰┌└][─━═╮╯┐┘\s]*|[─━═]{4,}\s*|│\s*)$`)
	// A box side with text after it: the hook runner's title box on a clean
	// run, but on a failing run it can carry the failure itself ("│ FAIL: ..."),
	// so it is cut only on a zero exit, and never when it carries a warning or
	// a danger word (a hook can warn and still pass).
	reGitRemoteBoxText = lazyre.New(`^\s*│.*$`)
)

// CompressGitRemote cuts only noise from a remote run and keeps every other
// line byte for byte, so each ref update, "error:", "rejected", "fatal:" and
// "remote:" line, hook step, hook summary and failing test (with its output)
// survives. The noise is: object-transfer progress, clean go test package
// lines ("ok pkg 0.3s", "? pkg [no test files]"), hook banner boxes and
// rules, and the blank lines they leave. On a zero exit the dropped package
// lines leave one "go test: N packages ok" line. Output it cannot read as plain
// text, or where nothing is gained, comes back unchanged with Applied false
// (fail open).
func CompressGitRemote(input []byte, exitCode int) Result {
	res := Result{Output: input, Format: FormatGitRemote, OriginalBytes: len(input), ReducedBytes: len(input)}
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
	out := dropGitRemoteNoise(string(text), exitCode == 0)
	if len(out) >= len(input) {
		res.Reason = "no_compression_gain"
		return res
	}
	res.Applied = true
	res.Output = []byte(out)
	res.ReducedBytes = len(out)
	res.Reason = "compressed remote output (refs, errors and hook results kept verbatim)"
	return res
}

// dropGitRemoteNoise removes the noise lines and any blank line that follows
// one, or another blank line. With summarize (a zero exit), box lines with
// text are cut too, and the clean go test package lines leave one count line
// where the first of them stood.
func dropGitRemoteNoise(body string, summarize bool) string {
	var b strings.Builder
	skipBlank := true
	pkgs, pkgAt := 0, -1
	for _, line := range strings.Split(strings.TrimSuffix(body, "\n"), "\n") {
		switch {
		case strings.TrimSpace(line) == "":
			if skipBlank {
				continue
			}
			skipBlank = true
		case reGoTestRunLine.MatchString(line):
			if pkgs == 0 {
				pkgAt = b.Len()
			}
			if strings.HasPrefix(line, "ok") {
				pkgs++
			}
			skipBlank = true
			continue
		case isGitRemoteNoise(line), summarize && reGitRemoteBoxText.MatchString(line) && !reKeepLine.MatchString(line):
			skipBlank = true
			continue
		default:
			skipBlank = false
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	out := b.String()
	if summarize && pkgs > 0 {
		out = out[:pkgAt] + fmt.Sprintf("go test: %d packages ok\n", pkgs) + out[pkgAt:]
	}
	return out
}

func isGitRemoteNoise(line string) bool {
	return reGitRemoteProgress.MatchString(line) || reGitRemoteBanner.MatchString(line)
}

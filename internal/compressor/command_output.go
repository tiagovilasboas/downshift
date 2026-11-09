// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"bytes"
	"strings"
	"unicode/utf8"
)

// CompressCommandOutput compacts the output of a command whose family the caller
// knows (ls, find, grep). The command names the format, not the text: a plain
// list of names has no shape that tells ls from find. The guards match
// CompressExit: a non-zero exit, empty, oversized, binary or non-UTF-8 input
// stays raw, and the text goes through the same colour and newline screen.
// Output the family's compressor does not recognise, or where nothing is gained,
// comes back unchanged with Applied false (fail open).
func CompressCommandOutput(input []byte, f Format, exitCode int) Result {
	return CompressCommandOutputHint(input, f, exitCode, CommandHint{})
}

// CommandHint is what the caller knows about the command that the output
// alone does not say. Only the file-read and path:text row formats read it.
type CommandHint struct {
	// Name is the file the command read, already shell-quoted for display in a
	// follow-up command; its extension decides whether the file is source.
	Name string
	// First is the file line number of the first output line (0 means 1).
	First int
	// Tail is true when the output is the end of the file (tail -n N): its line
	// numbers then count from the start of the output.
	Tail bool
	// Roots are the path operands of a grep or rg run. A row is read as
	// path:text only when its path is one of them or lies below one.
	Roots []string
	// HTTPHeaders is true when a curl run prints the response headers (-i, -I).
	HTTPHeaders bool
	// JQ is true when the command was `jq . FILE`: its output is JSON whatever
	// the file's extension.
	JQ bool
}

// CompressCommandOutputHint is CompressCommandOutput for the formats that
// need a CommandHint (file_read, grep_rows); the others ignore it.
func CompressCommandOutputHint(input []byte, f Format, exitCode int, h CommandHint) Result {
	res := Result{Output: input, Format: f, OriginalBytes: len(input), ReducedBytes: len(input)}
	switch {
	case exitCode > 0:
		res.Reason = "nonzero_exit_preserved"
		return res
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
	text, reason := screenCommandText(input, f)
	if reason != "" {
		res.Reason = reason
		return res
	}
	out, applied, reason := applyCommandFormat(text, f, h)
	res.Reason = reason
	if !applied {
		return res
	}
	res.Output, res.Applied, res.ReducedBytes = out, true, len(out)
	return res
}

// screenCommandText is screenText, except for an HTTP response: its headers
// end each line in CRLF and its body need not end in a newline, and neither is
// a reason to keep the response raw. Any escape byte still is.
func screenCommandText(input []byte, f Format) ([]byte, string) {
	if f != FormatHTTPFetch {
		return screenText(input)
	}
	if bytes.IndexByte(input, 0x1b) != -1 {
		return nil, "terminal_escape_preserved"
	}
	return input, ""
}

// commandLines splits text that ends in a newline into its lines.
func commandLines(text []byte) []string {
	return strings.Split(string(bytes.TrimSuffix(text, []byte("\n"))), "\n")
}

// applyCommandFormat routes the formats that read a CommandHint and sends the
// rest to applyFormatCompression.
func applyCommandFormat(text []byte, f Format, h CommandHint) ([]byte, bool, string) {
	switch f {
	case FormatGlab:
		return compressGlab(text)
	case FormatAgentDesktop:
		return compressAgentDesktop(text)
	case FormatHTTPFetch:
		return compressHTTPFetch(text, h)
	case FormatFileRead:
		return compressFileRead(text, h)
	case FormatGrepRows:
		return compressGrepRows(text, h)
	case FormatGrepCounts:
		return compressGrepCounts(text)
	}
	return applyFormatCompression(text, f)
}

// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package hookport

// ToolResultSample is one tool result the model received, as a harness's
// transcript records it: a CCR id found in its text and the UTF-8 length of
// that text. Never the text itself.
type ToolResultSample struct {
	ID    string
	Bytes int
}

// ReadToolResultsFunc looks for the given CCR ids in the tool results the
// model received and returns one sample per result that names one of them.
// ok is false when no transcript could be read (none exists, none is
// supported, an error): the caller then treats every row as unverified. The
// read is bounded, in memory only, and fails open.
type ReadToolResultsFunc func(ids []string) (samples []ToolResultSample, ok bool)

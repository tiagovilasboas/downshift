// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package compressor provides a deterministic, zero-dependency Go compressor
// for recognized tool outputs (git, tests, searches, file listings, logs).
// It never relies on an LLM, requires no network or Python, and guarantees
// complete preservation of errors, panics, and stack traces.
package compressor

// Mode defines the compression operating mode.
type Mode string

const (
	// ModeOff disables compression completely.
	ModeOff Mode = "off"
	// ModeObserve estimates savings and detects format without modifying the output.
	ModeObserve Mode = "observe"
	// ModeSafe compresses recognized formats only when provably safe (e.g. 0 failures).
	ModeSafe Mode = "safe"
)

// Format identifies the recognized output structure.
type Format string

const (
	FormatUnknown   Format = "unknown"
	FormatGitStatus Format = "git_status"
	FormatGitLog    Format = "git_log"
	FormatGoTest    Format = "go_test"
	FormatSearch    Format = "search"       // rg, grep
	FormatFileList  Format = "file_listing" // ls -R, tree, find
	FormatLogs      Format = "logs"         // repetitive log lines
)

// Result is the deterministic outcome of a compression attempt.
type Result struct {
	Output        []byte `json:"-"`
	Applied       bool   `json:"applied"`
	Format        Format `json:"format"`
	OriginalBytes int    `json:"original_bytes"`
	ReducedBytes  int    `json:"reduced_bytes"`
	Reason        string `json:"reason"`
}

// SavingsRatio returns the percentage of bytes reduced (0.0 to 100.0).
func (r Result) SavingsRatio() float64 {
	if r.OriginalBytes == 0 || r.ReducedBytes >= r.OriginalBytes {
		return 0.0
	}
	return float64(r.OriginalBytes-r.ReducedBytes) / float64(r.OriginalBytes) * 100.0
}

// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import "strings"

// Message parsing and formatting helpers for the gh run view --log compressor.
// They turn a raw log line into the text the summary keeps: the timestamp is
// stripped, known noise warnings are filtered, and short string helpers back
// the de-duplication and truncation the summary relies on.

// stripTimestamp removes the ISO8601 timestamp prefix from a log message.
// Format: 2026-10-24T12:54:19.3944192Z <message>
// Some lines have a BOM marker (U+FEFF) before the timestamp.
func stripTimestamp(s string) string {
	s = strings.TrimPrefix(s, "\ufeff") // Remove BOM
	s = strings.TrimSpace(s)

	// Match ISO8601 timestamp at start
	if len(s) > 28 && s[4] == '-' && s[7] == '-' && s[10] == 'T' {
		// Find the end of timestamp (after the Z)
		if idx := strings.Index(s, "Z "); idx > 0 && idx < 30 {
			return strings.TrimSpace(s[idx+2:])
		}
		if idx := strings.Index(s, "Z"); idx > 0 && idx < 30 && idx == len(s)-1 {
			return "" // Just a timestamp, no message
		}
	}
	return s
}

// isNoiseWarning returns true for common warnings that don't need agent attention.
func isNoiseWarning(msg string) bool {
	noise := []string{
		"ubuntu-latest label will migrate",
		"Node.js 16 actions are deprecated",
		"set-output command is deprecated",
		"save-state command is deprecated",
		"group command is deprecated",
	}
	for _, n := range noise {
		if strings.Contains(strings.ToLower(msg), strings.ToLower(n)) {
			return true
		}
	}
	return false
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

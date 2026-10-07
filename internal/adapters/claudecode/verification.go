// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"regexp"
)

// Verification values recorded on a usage event.
const (
	verificationPassed = "passed"
	verificationFailed = "failed"
	verificationNone   = "none"
)

// verifyCommand matches the shell commands that count as a subagent checking
// its own work: test runners, linters and type checkers. It is deliberately a
// short allowlist; an unrecognised command counts as no verification.
var verifyCommand = regexp.MustCompile(`(?:^|[\s;&|(])(?:go\s+(?:test|vet)|pytest|python3?\s+-m\s+(?:pytest|unittest)|npm\s+(?:run\s+)?(?:test|lint)|pnpm\s+(?:run\s+)?(?:test|lint)|yarn\s+(?:test|lint)|vitest|jest|cargo\s+(?:test|clippy)|tsc|eslint|ruff|golangci-lint|make\s+(?:test|lint|check))(?:\s|$)`)

// readSubagentVerification reports whether a subagent verified its own work.
// It pairs each recognised check command (a Bash tool_use) with the matching
// tool_result and reads only the is_error flag; the command text and the output
// are never kept. The last check wins, since a subagent that fixes a failure and
// reruns the check ends on its final state. Any problem reads as "none": this
// is a sensor on the hook path and must fail open.
func readSubagentVerification(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return verificationNone
	}
	defer f.Close()
	if fi, err := f.Stat(); err != nil || !fi.Mode().IsRegular() || fi.Size() > maxTranscriptBytes {
		return verificationNone
	}

	pending := map[string]bool{} // tool_use id of a check awaiting its result
	result := verificationNone
	rd := bufio.NewReaderSize(io.LimitReader(f, maxTranscriptBytes), 1<<20)
	for {
		line, err := rd.ReadBytes('\n')
		if len(line) > 0 {
			var row struct {
				Message struct {
					Content json.RawMessage `json:"content"`
				} `json:"message"`
			}
			if json.Unmarshal(line, &row) == nil {
				var blocks []struct {
					Type      string `json:"type"`
					ID        string `json:"id"`
					Name      string `json:"name"`
					ToolUseID string `json:"tool_use_id"`
					IsError   bool   `json:"is_error"`
					Input     struct {
						Command string `json:"command"`
					} `json:"input"`
				}
				if json.Unmarshal(row.Message.Content, &blocks) == nil {
					for _, b := range blocks {
						switch b.Type {
						case "tool_use":
							if b.Name == "Bash" && b.ID != "" && verifyCommand.MatchString(b.Input.Command) {
								pending[b.ID] = true
							}
						case "tool_result":
							if pending[b.ToolUseID] {
								delete(pending, b.ToolUseID)
								if b.IsError {
									result = verificationFailed
								} else {
									result = verificationPassed
								}
							}
						}
					}
				}
			}
		}
		if err != nil {
			break
		}
	}
	return result
}

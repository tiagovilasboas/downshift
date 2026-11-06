// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strings"
)

// ghPrBodyMax caps the PR description kept by the gh pr view compressor.
const ghPrBodyMax = 30

// ghPrBody returns the description that follows the "--" separator of a
// non-TTY `gh pr view`, capped at ghPrBodyMax lines. A cut is never silent: the
// last line says how many lines were left out.
func ghPrBody(s string) string {
	_, body, found := strings.Cut(s, "\n--\n")
	if !found {
		return ""
	}
	lines := strings.Split(strings.Trim(body, "\n"), "\n")
	if len(lines) == 1 && strings.TrimSpace(lines[0]) == "" {
		return ""
	}
	if len(lines) > ghPrBodyMax {
		cut := len(lines) - ghPrBodyMax
		lines = append(lines[:ghPrBodyMax], fmt.Sprintf("[… %d lines cut]", cut))
	}
	return strings.Join(lines, "\n") + "\n"
}

// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package semantic

import (
	"os"
	"strings"
)

// Embedder produces a dense vector for a prompt.
type Embedder interface {
	Embed(prompt string) ([]float64, error)
}

// enabled is on unless DOWNSHIFT_MINILM is 0, false, or off.
func enabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("DOWNSHIFT_MINILM")))
	switch v {
	case "0", "false", "off", "no":
		return false
	default:
		return true
	}
}

// Enabled reports whether the semantic boost runs (DOWNSHIFT_MINILM is not
// 0, false, off or no). It is read-only, for `downshift config`.
func Enabled() bool { return enabled() }

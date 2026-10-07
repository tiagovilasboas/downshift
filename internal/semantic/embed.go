// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package semantic

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift/internal/hookctx"
)

// Embedder produces a dense vector for a prompt.
type Embedder interface {
	Embed(prompt string) ([]float64, error)
}

// CmdEmbedder runs an external command (MiniLM via Python) that reads the
// prompt on stdin and prints a JSON array of floats on stdout.
type CmdEmbedder struct {
	Cmd string
}

func (c CmdEmbedder) Embed(prompt string) ([]float64, error) {
	if strings.TrimSpace(c.Cmd) == "" {
		return nil, fmt.Errorf("empty embed command")
	}
	// Bounded by embedCmdTimeout and the hook's global deadline; output is
	// capped. A hung helper falls back instead of stalling the spawn.
	out, err := hookctx.RunCommand(c.Cmd, prompt, embedCmdTimeout)
	if err != nil {
		return nil, fmt.Errorf("embed command failed: %w", err)
	}
	var vec []float64
	if err := json.Unmarshal(out, &vec); err != nil {
		return nil, fmt.Errorf("embed output not json array: %w", err)
	}
	return vec, nil
}

// embedCmdTimeout bounds one DOWNSHIFT_MINILM_EMBED call.
const embedCmdTimeout = time.Second

type fallbackEmbedder struct {
	primary  Embedder
	fallback Embedder
}

func (f fallbackEmbedder) Embed(prompt string) ([]float64, error) {
	vec, _, err := f.embedSource(prompt)
	return vec, err
}

// embedSource is Embed that also reports whether the primary (external)
// embedder produced the vector, so callers can pick centroids from the same
// embedding space.
func (f fallbackEmbedder) embedSource(prompt string) (vec []float64, primary bool, err error) {
	if f.primary != nil {
		if v, perr := f.primary.Embed(prompt); perr == nil && len(v) > 0 {
			return v, true, nil
		}
	}
	if f.fallback == nil {
		return nil, false, fmt.Errorf("no embedder")
	}
	vec, err = f.fallback.Embed(prompt)
	return vec, false, err
}

// EmbedderFromEnv returns the local hash embedder by default.
// DOWNSHIFT_MINILM_EMBED=<cmd> tries that command first and falls back to hash
// when the command is missing, fails, or returns an empty vector.
func EmbedderFromEnv() (Embedder, bool) {
	hash := HashEmbedder{}
	cmd := strings.TrimSpace(os.Getenv("DOWNSHIFT_MINILM_EMBED"))
	if cmd == "" || cmd == "hash" {
		return hash, true
	}
	return fallbackEmbedder{primary: CmdEmbedder{Cmd: cmd}, fallback: hash}, true
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

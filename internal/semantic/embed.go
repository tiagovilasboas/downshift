// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1

package semantic

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
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
	if c.Cmd == "" {
		return nil, fmt.Errorf("empty embed command")
	}
	parts := strings.Fields(c.Cmd)
	if len(parts) == 0 {
		return nil, fmt.Errorf("invalid embed command")
	}
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Stdin = strings.NewReader(prompt)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("embed command failed: %w", err)
	}
	var vec []float64
	if err := json.Unmarshal(out, &vec); err != nil {
		return nil, fmt.Errorf("embed output not json array: %w", err)
	}
	return vec, nil
}

// fallbackEmbedder tries primary (optional external MiniLM) then the local hash embedder.
type fallbackEmbedder struct {
	primary  Embedder
	fallback Embedder
}

func (f fallbackEmbedder) Embed(prompt string) ([]float64, error) {
	if f.primary != nil {
		if vec, err := f.primary.Embed(prompt); err == nil && len(vec) > 0 {
			return vec, nil
		}
	}
	if f.fallback == nil {
		return nil, fmt.Errorf("no embedder")
	}
	return f.fallback.Embed(prompt)
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

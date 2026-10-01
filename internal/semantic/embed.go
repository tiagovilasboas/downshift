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

// EmbedderFromEnv returns a CmdEmbedder when DOWNSHIFT_MINILM_EMBED is set.
func EmbedderFromEnv() (Embedder, bool) {
	cmd := strings.TrimSpace(os.Getenv("DOWNSHIFT_MINILM_EMBED"))
	if cmd == "" {
		return HashEmbedder{}, true
	}
	if cmd == "hash" {
		return HashEmbedder{}, true
	}
	return CmdEmbedder{Cmd: cmd}, true
}

func enabled() bool {
	v := strings.TrimSpace(os.Getenv("DOWNSHIFT_MINILM"))
	return v == "1" || strings.EqualFold(v, "true")
}

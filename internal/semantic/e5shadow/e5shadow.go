// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package e5shadow observes multilingual E5 embeddings in parallel with production
// classification. It never changes Route() outcomes unless a separate integration
// flag is added later; enable with DOWNSHIFT_E5_SHADOW=1.
//
// harness-downshift by Tiago de Carvalho Vilas Boas
// https://github.com/tiagovilasboas/downshift
package e5shadow

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift/internal/semantic"
)

const (
	// EnableEnv turns on shadow observation (default off).
	EnableEnv = "DOWNSHIFT_E5_SHADOW"
	// CmdEnv is the embed helper: reads prompt on stdin, prints a JSON float array.
	CmdEnv = "DOWNSHIFT_E5_SHADOW_CMD"
	// PrototypesEnv points at a centroids JSON file (PrototypeStore shape).
	PrototypesEnv = "DOWNSHIFT_E5_PROTOTYPES"
)

// Observation is prompt-free telemetry for shadow comparison.
type Observation struct {
	ModelID        string
	NearestLabel   string
	Similarity     float64
	WouldLabel     string
	WouldLift      bool
	RegexLabel     string
	RegexConfident bool
	LatencyMs      float64
	ErrorCode      string
}

// Enabled reports whether shadow observation is requested for this process.
func Enabled() bool {
	return strings.TrimSpace(os.Getenv(EnableEnv)) == "1"
}

// Observe runs the E5 shadow path when Enabled(). It never persists prompt text.
// Returns nil when shadow is disabled.
func Observe(prompt, regexLabel string, regexConfident bool) (o *Observation) {
	if !Enabled() {
		return nil
	}
	start := time.Now()
	o = &Observation{
		RegexLabel:     regexLabel,
		RegexConfident: regexConfident,
	}
	defer func() {
		o.LatencyMs = float64(time.Since(start).Milliseconds())
		if recover() != nil && o.ErrorCode == "" {
			o.ErrorCode = "observe_panic"
		}
	}()

	cmd := strings.TrimSpace(os.Getenv(CmdEnv))
	if cmd == "" {
		o.ErrorCode = "missing_cmd"
		return o
	}
	if !CmdTrusted(cmd) {
		o.ErrorCode = "untrusted_cmd"
		return o
	}
	store, modelID, err := loadPrototypes(os.Getenv(PrototypesEnv))
	if err != nil {
		o.ErrorCode = "load_prototypes"
		return o
	}
	o.ModelID = modelID

	emb := semantic.CmdEmbedder{Cmd: cmd}
	vec, err := emb.Embed(prompt)
	if err != nil {
		o.ErrorCode = "embed_error"
		return o
	}
	nearest, sim := store.Nearest(vec)
	o.NearestLabel = nearest
	o.Similarity = sim

	wouldLabel, lifted := semantic.AugmentVector(vec, regexLabel, regexConfident, store)
	o.WouldLabel = wouldLabel
	o.WouldLift = lifted
	return o
}

const maxPrototypeBytes = 256 << 10 // 256 KiB; centroids only, no prompts.

func loadPrototypes(path string) (semantic.PrototypeStore, string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return semantic.PrototypeStore{}, "", fmt.Errorf("empty prototypes path")
	}
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return semantic.PrototypeStore{}, "", fmt.Errorf("prototypes must be a regular file")
	}
	if info.Size() > maxPrototypeBytes {
		return semantic.PrototypeStore{}, "", fmt.Errorf("prototypes too large")
	}
	f, err := os.Open(path)
	if err != nil {
		return semantic.PrototypeStore{}, "", err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxPrototypeBytes+1))
	if err != nil || len(data) > maxPrototypeBytes {
		return semantic.PrototypeStore{}, "", fmt.Errorf("read prototypes")
	}
	var store semantic.PrototypeStore
	if err := json.Unmarshal(data, &store); err != nil {
		return semantic.PrototypeStore{}, "", err
	}
	if err := store.Validate(); err != nil {
		return semantic.PrototypeStore{}, "", err
	}
	digest := sha256.Sum256(data)
	id := "e5-prototypes-sha256:" + hex.EncodeToString(digest[:])
	return store, id, nil
}

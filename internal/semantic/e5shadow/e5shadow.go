// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package e5shadow observes multilingual E5 embeddings in parallel with production
// classification and, when a separate fuse flag is on, supplies the same lookup
// to ClassifyWithSemantic. Shadow never changes Route() by itself; enable with
// DOWNSHIFT_E5_SHADOW=1. Production fuse is DOWNSHIFT_E5_FUSE=1 (default off, fail-open).
//
// Downshift by Tiago de Carvalho Vilas Boas
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
	"sync"
	"time"

	"github.com/tiagovilasboas/downshift/internal/semantic"
)

const (
	// EnableEnv turns on shadow observation (default off).
	EnableEnv = "DOWNSHIFT_E5_SHADOW"
	// IntegrateEnv turns on the production fuse in ClassifyWithSemantic (default off).
	// Distinct from EnableEnv: shadow never rewrites tier; this flag may.
	IntegrateEnv = "DOWNSHIFT_E5_FUSE"
	// CmdEnv is the shadow embed helper: reads prompt on stdin, prints a JSON float array.
	CmdEnv = "DOWNSHIFT_E5_SHADOW_CMD"
	// CmdEnvAlias is the fuse helper. Fuse does not fall back to CmdEnv.
	CmdEnvAlias = "DOWNSHIFT_E5_CMD"
	// PrototypesEnv points at a centroids JSON file (PrototypeStore shape).
	PrototypesEnv = "DOWNSHIFT_E5_PROTOTYPES"
	// LabelAbstain / LabelForeign are reserved prototype labels (same strings
	// as semantic.LabelAbstain / LabelForeign).
	LabelAbstain = semantic.LabelAbstain
	LabelForeign = semantic.LabelForeign
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

// Lookup is an in-memory nearest-centroid result. The vector stays in process
// memory and is never written to telemetry or disk.
type Lookup struct {
	NearestLabel string
	Similarity   float64
	ModelID      string
	ErrorCode    string
	Vec          []float64
	Store        semantic.PrototypeStore
}

// Enabled reports whether shadow observation is requested for this process.
func Enabled() bool {
	return strings.TrimSpace(os.Getenv(EnableEnv)) == "1"
}

// IntegrateEnabled reports whether the production E5 fuse is requested.
// Only the exact value "1" is on; true/yes/on are treated as off.
func IntegrateEnabled() bool {
	return strings.TrimSpace(os.Getenv(IntegrateEnv)) == "1"
}

// FuseCmdFrom returns DOWNSHIFT_E5_CMD only (no shadow-cmd fallback).
func FuseCmdFrom(getenv func(string) string) string {
	if getenv == nil {
		return ""
	}
	return strings.TrimSpace(getenv(CmdEnvAlias))
}

// ShadowCmdFrom returns DOWNSHIFT_E5_SHADOW_CMD only.
func ShadowCmdFrom(getenv func(string) string) string {
	if getenv == nil {
		return ""
	}
	return strings.TrimSpace(getenv(CmdEnv))
}

// EmbedCmdFrom is the doctor helper: fuse cmd when fuse is on, else shadow cmd.
func EmbedCmdFrom(getenv func(string) string) string {
	if getenv == nil {
		return ""
	}
	if strings.TrimSpace(getenv(IntegrateEnv)) == "1" {
		return FuseCmdFrom(getenv)
	}
	return ShadowCmdFrom(getenv)
}

// LanguageAbstain reports reserved centroids that clear confidence only.
func LanguageAbstain(label string) bool {
	return label == LabelAbstain || label == LabelForeign
}

type lookupCache struct {
	mu     sync.Mutex
	key    string
	result *Lookup
}

// lastLookup reuses one embed for fuse + shadow on the same prompt/cmd/prototypes.
// The prompt is held only as a cache key in this process; it is never persisted.
var lastLookup lookupCache

func cacheKey(prompt, cmd, proto string) string {
	return cmd + "\x00" + proto + "\x00" + prompt
}

// LookupNearest embeds using the shadow command (DOWNSHIFT_E5_SHADOW_CMD).
func LookupNearest(prompt string) *Lookup {
	return lookupNearestWith(prompt, ShadowCmdFrom(os.Getenv))
}

// LookupFuse embeds using the fuse command (DOWNSHIFT_E5_CMD). No shadow fallback.
func LookupFuse(prompt string) *Lookup {
	return lookupNearestWith(prompt, FuseCmdFrom(os.Getenv))
}

func lookupNearestWith(prompt, cmd string) *Lookup {
	proto := strings.TrimSpace(os.Getenv(PrototypesEnv))
	key := cacheKey(prompt, cmd, proto)

	lastLookup.mu.Lock()
	defer lastLookup.mu.Unlock()
	if lastLookup.result != nil && lastLookup.key == key {
		return lastLookup.result
	}
	result := computeLookup(prompt, cmd, proto)
	lastLookup.key = key
	lastLookup.result = result
	return result
}

func computeLookup(prompt, cmd, proto string) *Lookup {
	out := &Lookup{}
	if cmd == "" {
		out.ErrorCode = "missing_cmd"
		return out
	}
	if !CmdTrusted(cmd) {
		out.ErrorCode = "untrusted_cmd"
		return out
	}
	store, modelID, err := loadPrototypes(proto)
	if err != nil {
		out.ErrorCode = "load_prototypes"
		return out
	}
	out.ModelID = modelID
	out.Store = store

	emb := semantic.CmdEmbedder{Cmd: cmd}
	vec, err := emb.Embed(prompt)
	if err != nil || len(vec) == 0 {
		out.ErrorCode = "embed_error"
		return out
	}
	nearest, sim := store.Nearest(vec)
	out.NearestLabel = nearest
	out.Similarity = sim
	out.Vec = vec
	return out
}

// Observe runs the E5 shadow path when Enabled(). It never persists prompt text.
// Returns nil when shadow is disabled. Shares LookupNearest with the production fuse.
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

	look := LookupNearest(prompt)
	if look == nil {
		o.ErrorCode = "embed_error"
		return o
	}
	o.ModelID = look.ModelID
	if look.ErrorCode != "" {
		o.ErrorCode = look.ErrorCode
		return o
	}
	o.NearestLabel = look.NearestLabel
	o.Similarity = look.Similarity

	wouldLabel, lifted := semantic.AugmentVector(look.Vec, regexLabel, regexConfident, look.Store)
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

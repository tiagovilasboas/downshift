// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// The squad's fan-out cap: how many independent parts one wave may run in
// parallel. It is data, not prose: orchestrators read the effective value from
// the MCP playbook. Precedence, highest first: the user's own override (the
// DOWNSHIFT_MAX_PARALLEL variable), a value a harness declares, the policy
// file ("orchestration": {"max_parallel": N}), the default.
const (
	// DefaultMaxParallel is 6. No code path shows a hard concurrency limit on
	// any harness, and each extra slice adds its own context and quota use, so
	// the default stays small and every layer above it can override it.
	DefaultMaxParallel = 6
	// MaxParallelLimit bounds a configured value; above it is a typo.
	MaxParallelLimit = 64
	// ParallelEnv is the user's override.
	ParallelEnv = "DOWNSHIFT_MAX_PARALLEL"
)

// Sources of an effective cap, highest precedence first.
const (
	ParallelFromUser    = "user"
	ParallelFromHarness = "harness"
	ParallelFromPolicy  = "policy"
	ParallelFromDefault = "default"
)

// Parallel is the effective cap and where it came from. WhenStrained is the
// cap to use while the quota advice reports strain. Notes lists each ignored
// invalid value, so a bad setting is visible, never silent.
type Parallel struct {
	Max          int      `json:"max"`
	WhenStrained int      `json:"when_strained"`
	Source       string   `json:"source"`
	Notes        []string `json:"notes,omitempty"`
}

// ResolveParallel applies the precedence. A zero value means "not set".
func ResolveParallel(user, harness, policy int, notes []string) Parallel {
	p := Parallel{Max: DefaultMaxParallel, Source: ParallelFromDefault, Notes: notes}
	switch {
	case user > 0:
		p.Max, p.Source = user, ParallelFromUser
	case harness > 0:
		p.Max, p.Source = harness, ParallelFromHarness
	case policy > 0:
		p.Max, p.Source = policy, ParallelFromPolicy
	}
	p.WhenStrained = ShrinkForStrain(p.Max)
	return p
}

// ShrinkForStrain halves a cap (rounding down) while quota is rate limited or
// low on headroom. It never goes below 1.
func ShrinkForStrain(n int) int {
	if n/2 < 1 {
		return 1
	}
	return n / 2
}

// EnvParallel reads the user's override. An invalid value is no override and
// returns a note.
func EnvParallel(getenv func(string) string) (int, []string) {
	raw := strings.TrimSpace(getenv(ParallelEnv))
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > MaxParallelLimit {
		return 0, []string{fmt.Sprintf("%s=%q ignored: want an integer from 1 to %d", ParallelEnv, raw, MaxParallelLimit)}
	}
	return n, nil
}

// FilesParallel reads orchestration.max_parallel from the project file, then
// the user file. A file with an invalid value is skipped with a note and the
// next file is tried; an unreadable or absent file holds no value. It never
// fails: the cap falls back to the default.
func FilesParallel(loc Locations) (int, []string) {
	var notes []string
	for _, path := range []string{loc.Project, loc.User} {
		n, note := fileParallel(path)
		if note != "" {
			notes = append(notes, note)
		}
		if n > 0 {
			return n, notes
		}
	}
	return 0, notes
}

func fileParallel(path string) (int, string) {
	if path == "" {
		return 0, ""
	}
	fi, err := os.Lstat(path)
	if err != nil || !fi.Mode().IsRegular() || fi.Size() > maxBytes {
		return 0, ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, ""
	}
	var doc struct {
		Orchestration json.RawMessage `json:"orchestration"`
	}
	if json.Unmarshal(data, &doc) != nil || doc.Orchestration == nil {
		return 0, ""
	}
	var o struct {
		MaxParallel json.RawMessage `json:"max_parallel"`
	}
	if json.Unmarshal(doc.Orchestration, &o) != nil {
		return 0, path + ": orchestration must be an object; max_parallel ignored"
	}
	if o.MaxParallel == nil {
		return 0, ""
	}
	raw := strings.TrimSpace(string(o.MaxParallel))
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > MaxParallelLimit {
		return 0, fmt.Sprintf("%s: orchestration.max_parallel %s ignored: want an integer from 1 to %d", path, raw, MaxParallelLimit)
	}
	return n, ""
}

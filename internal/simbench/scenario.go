// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package simbench is the offline simulation layer: it drives synthetic spawns
// through the real shared hook path, ends each child with synthetic usage, and
// reads the resulting event log back through the production aggregation
// (telemetry.Summarize, Aggregate, ExportSummary and BuildReport). It makes no
// model call and opens no network connection.
//
// What it proves: routing decisions, rewrites, holds, control-group linkage
// and savings accounting are internally consistent for the scenarios in
// testdata/sim. What it does not prove: quality, or real token counts. Every
// dollar figure it prints is an ESTIMATE on evidence rungs A/B
// (docs/guides/evidence.md), never a measurement.
//
// The package defines the Driver port; the binary's shared runner implements it
// in cmd/downshift (simbench_test.go), so simbench never imports an adapter
// and the hook runner stays where it is.
//
// Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift
package simbench

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
)

//go:embed testdata/sim/*.json
var scenarioFS embed.FS

// Range is an inclusive [min, max] token count drawn uniformly.
type Range [2]int64

// Profile is the synthetic token usage of one child, per token class.
type Profile struct {
	Input      Range `json:"input"`
	Output     Range `json:"output"`
	CacheRead  Range `json:"cache_read"`
	CacheWrite Range `json:"cache_write"`
}

// Expect is what a scenario must show on one harness. Shares are over routed
// (non-control) spawns. A nil pointer means "not checked".
type Expect struct {
	// MinRewriteShare and MaxRewriteShare bound the share of routed spawns the
	// hook wrote a model for.
	MinRewriteShare *float64 `json:"min_rewrite_share,omitempty"`
	MaxRewriteShare *float64 `json:"max_rewrite_share,omitempty"`
	// NoDownshift: no routed spawn may run on a cheaper model than requested.
	NoDownshift bool `json:"no_downshift,omitempty"`
	// UnknownBaseline: every decision must record an unknown requested model.
	UnknownBaseline bool `json:"unknown_baseline,omitempty"`
}

// Scenario is one simulated workload: the same task mix and usage profile run
// on every listed harness.
type Scenario struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	// Seed makes ids, usage draws and therefore the control share
	// deterministic. Zero is replaced by DefaultSeed.
	Seed uint64 `json:"seed"`
	// Harnesses to run. Empty means all of Harnesses().
	Harnesses []string `json:"harnesses"`
	// ParentTier is the tier of the model the parent session runs (small, mid
	// or frontier); the requested model is that harness's catalog model for it.
	ParentTier string `json:"parent_tier"`
	// ExplicitModel puts the requested model in the spawn call itself. False
	// leaves it to inheritance where the harness supports it (Claude Code reads
	// the parent transcript); harnesses without inheritance always name it.
	ExplicitModel bool `json:"explicit_model"`
	// AgentType is the subagent type (Claude Code subagent_type, OpenCode
	// agent). Empty means general-purpose.
	AgentType string `json:"agent_type"`
	// Spawns per harness.
	Spawns int `json:"spawns"`
	// Tasks are cycled over the spawns. "{canary}" is replaced by a unique
	// token per spawn so the prompt-free check can look for it.
	Tasks []string `json:"tasks"`
	// Usage is the token profile every child draws from.
	Usage Profile `json:"usage"`
	// Refuse makes the harness ignore every rewrite: the child runs on the
	// requested model and reports it. A refuted rewrite must save nothing.
	Refuse bool `json:"refuse_rewrites"`
	// Expect holds the per-harness expectations; key "*" applies to every
	// harness without its own entry.
	Expect map[string]Expect `json:"expect"`
}

// DefaultSeed is the seed of a scenario that names none.
const DefaultSeed = 20261019

// Canary is the prefix of the per-spawn token planted in task text.
const Canary = "simcanary"

// ExpectFor returns the expectation for a harness.
func (s Scenario) ExpectFor(harness string) Expect {
	if e, ok := s.Expect[harness]; ok {
		return e
	}
	return s.Expect["*"]
}

// Validate reports the first structural problem of a scenario.
func (s Scenario) Validate() error {
	switch {
	case s.Name == "":
		return fmt.Errorf("scenario without a name")
	case s.Spawns <= 0:
		return fmt.Errorf("%s: spawns must be positive", s.Name)
	case len(s.Tasks) == 0:
		return fmt.Errorf("%s: no tasks", s.Name)
	}
	if _, ok := tierByName[s.ParentTier]; !ok {
		return fmt.Errorf("%s: unknown parent_tier %q", s.Name, s.ParentTier)
	}
	for _, h := range s.Harnesses {
		if _, ok := wires[h]; !ok {
			return fmt.Errorf("%s: unknown harness %q", s.Name, h)
		}
	}
	for _, r := range []Range{s.Usage.Input, s.Usage.Output, s.Usage.CacheRead, s.Usage.CacheWrite} {
		if r[0] < 0 || r[1] < r[0] {
			return fmt.Errorf("%s: bad usage range %v", s.Name, r)
		}
	}
	if s.Usage.Input[1]+s.Usage.Output[1] == 0 {
		return fmt.Errorf("%s: usage profile bills nothing", s.Name)
	}
	for _, t := range s.Tasks {
		if !strings.Contains(t, "{canary}") {
			return fmt.Errorf("%s: task without {canary}: %q", s.Name, t)
		}
	}
	return nil
}

// harnessList is the harnesses this scenario runs.
func (s Scenario) harnessList() []string {
	if len(s.Harnesses) > 0 {
		return s.Harnesses
	}
	return Harnesses()
}

// Scenarios loads every embedded scenario, sorted by name.
func Scenarios() ([]Scenario, error) {
	return LoadFS(scenarioFS, "testdata/sim")
}

// LoadFS reads every *.json scenario in dir of fsys.
func LoadFS(fsys fs.FS, dir string) ([]Scenario, error) {
	names, err := fs.Glob(fsys, path.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	var out []Scenario
	for _, name := range names {
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, err
		}
		var sc Scenario
		dec := json.NewDecoder(strings.NewReader(string(data)))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&sc); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		if sc.Seed == 0 {
			sc.Seed = DefaultSeed
		}
		if err := sc.Validate(); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		out = append(out, sc)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package simbench

import (
	"fmt"
	"hash/fnv"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// Driver is the port to the real hook path. The binary implements it with its
// shared runners (runHookAdapter, runHonor, runEnd); simbench never calls an
// adapter itself. Each call is one hook invocation with one stdin payload.
type Driver interface {
	// Hook runs the spawn-time hook and returns its stdout and exit code.
	Hook(harness string, payload []byte) (stdout []byte, rc int)
	// Honor runs the post-launch observation hook (`downshift honor`).
	Honor(harness string, payload []byte)
	// End runs the child-end hook (`downshift end`).
	End(harness string, payload []byte)
}

// Trace is what the simulator sent and saw for one spawn: the ground truth the
// invariants hold the event log to.
type Trace struct {
	Spawn
	// Written is the model the hook wrote, "" when it left the spawn alone.
	Written string
	// Ran is the model the child ran on: Written unless the harness refused it.
	Ran string
	RC  int
	// Control is the production control-group draw for this routing id.
	Control bool
	// Usage is the child's tokens as the harness can report them.
	Usage telemetry.TokenUsage
}

var tierByName = map[string]core.Tier{"small": core.TierSmall, "mid": core.TierMid, "frontier": core.TierFrontier}

// Run plays the scenario on every listed harness through d, then reads the
// sandbox event log and checks it. The returned error is an infrastructure
// failure (a fixture could not be written, the log could not be read);
// accounting problems are Result.Violations.
func Run(sc Scenario, d Driver, env Env, cat core.Resolver) (Result, error) {
	rng := rand.New(rand.NewPCG(sc.Seed, nameHash(sc.Name)))
	var traces []Trace
	for _, h := range sc.harnessList() {
		w := wires[h]
		dir := filepath.Join(env.Fixtures, h)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return Result{}, err
		}
		parent := cat.ModelFor(h, tierByName[sc.ParentTier]).ID
		agentType := sc.AgentType
		if agentType == "" {
			agentType = "general-purpose"
		}
		for i := range sc.Spawns {
			s := Spawn{
				Harness:     h,
				Session:     fmt.Sprintf("%s-%s-%d", sessionPrefix, h, i/5),
				ToolUse:     toolPrefix + hexID(rng, 1),
				Agent:       agentPrefix + hexID(rng, 1),
				Correlation: hexID(rng, 2),
				Task:        strings.ReplaceAll(sc.Tasks[i%len(sc.Tasks)], "{canary}", Canary+hexID(rng, 1)[:8]),
				AgentType:   agentType,
				Requested:   parent,
				Explicit:    sc.ExplicitModel,
				Dir:         dir,
				ProjectDir:  env.Project,
			}
			if w.readsAgentDefinitions && agentType == AgentPinned && !s.Explicit {
				s.Requested = env.PinnedModel
			}
			if err := w.prepare(s); err != nil {
				return Result{}, err
			}
			stdout, rc := d.Hook(h, w.spawn(s))
			t := Trace{
				Spawn:   s,
				Written: w.written(stdout),
				RC:      rc,
				Control: telemetry.InControlGroup(s.Correlation, env.ControlShare),
				Usage:   w.reported(sc.Usage.draw(rng)),
			}
			t.Ran = s.Requested
			if t.Written != "" && !sc.Refuse {
				t.Ran = t.Written
			}
			if w.honor != nil && t.Written != "" {
				d.Honor(h, w.honor(s, t.Written, t.Ran))
			}
			ends, err := w.ends(s, t.Ran, t.Usage)
			if err != nil {
				return Result{}, err
			}
			for _, p := range ends {
				d.End(h, p)
			}
			traces = append(traces, t)
		}
	}
	events, err := telemetry.ReadEventsFrom(env.EventLog)
	if err != nil {
		return Result{}, fmt.Errorf("read sandbox event log: %w", err)
	}
	res := Result{Scenario: sc, Traces: traces, Events: events}
	res.Harnesses = summarize(sc, traces, events, cat)
	res.Violations = Check(sc, traces, events, cat)
	leaks, err := ScanPromptFree(traces, env.StateDir, env.Home, env.Project)
	if err != nil {
		return Result{}, err
	}
	res.Violations = append(res.Violations, leaks...)
	return res, nil
}

// draw picks one child's tokens. CachedTokens is read plus write, like the
// harness totals; CacheWriteTokens is the write part.
func (p Profile) draw(rng *rand.Rand) telemetry.TokenUsage {
	pick := func(r Range) int64 {
		if r[1] <= r[0] {
			return r[0]
		}
		return r[0] + rng.Int64N(r[1]-r[0]+1)
	}
	read, write := pick(p.CacheRead), pick(p.CacheWrite)
	return telemetry.TokenUsage{
		InputTokens:      pick(p.Input),
		OutputTokens:     pick(p.Output),
		CachedTokens:     read + write,
		CacheWriteTokens: write,
	}
}

func hexID(rng *rand.Rand, words int) string {
	var b strings.Builder
	for range words {
		fmt.Fprintf(&b, "%016x", rng.Uint64())
	}
	return b.String()
}

func nameHash(name string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(name))
	return h.Sum64()
}

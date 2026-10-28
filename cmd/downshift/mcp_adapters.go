// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift/internal/mcp/tools"
	"github.com/tiagovilasboas/downshift/internal/models"
	"github.com/tiagovilasboas/downshift/internal/playbook"
	"github.com/tiagovilasboas/downshift/internal/quota"
	"github.com/tiagovilasboas/downshift/internal/routing/contract"
	"github.com/tiagovilasboas/downshift/internal/routing/headroom"
)

// mcpModels adapts the detected-models resolver (K1) to the tools port.
type mcpModels struct{ md models.Metadata }

func (m mcpModels) Available(_ context.Context, harness string) (tools.ModelList, error) {
	l, err := models.Resolve(harness, m.md, models.Options{})
	if err != nil {
		return tools.ModelList{}, errors.New("unknown harness")
	}
	out := tools.ModelList{Harness: l.Harness, Fallback: l.Fallback, CacheWrittenAt: l.CacheWrittenAt, Models: []tools.Model{}}
	for _, a := range l.Models {
		out.Models = append(out.Models, tools.Model{ID: a.ID, Tier: a.Tier, Efforts: a.Efforts, Source: a.Source, Untiered: a.Untiered})
	}
	return out, nil
}

// mcpPlaybooks reads the active playbook sets from the state directory, so an
// edit made with `downshift playbook` applies to the next call.
type mcpPlaybooks struct{}

func (mcpPlaybooks) Sets() ([]playbook.Set, error) {
	dir, err := playbook.Dir()
	if err != nil {
		return nil, errors.New("the playbook folder cannot be located")
	}
	return playbook.Load(dir)
}

// mcpStrain reads the native quota snapshot of each harness and reports the
// fullest window as advice. A harness with no usable reading is "unknown".
type mcpStrain struct{ now func() time.Time }

func (s mcpStrain) Strain(_ context.Context, ids []string) ([]tools.StrainEntry, error) {
	var providers []contract.HarnessID
	var readings []contract.Reading
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		providers = append(providers, contract.HarnessID(id))
		if snap, _ := quota.Load(id); snap != nil {
			readings = append(readings, quota.Readings(*snap, 0)...)
		}
	}
	var out []tools.StrainEntry
	for _, v := range headroom.Assess(providers, readings, s.now(), headroom.DefaultCriticalThreshold) {
		name := string(v.Strain)
		if name == "" {
			name = "unknown"
		}
		out = append(out, tools.StrainEntry{Provider: string(v.Harness), Window: string(v.Window), UsedPct: v.UsedPct, RateLimited: v.RateLimited, Headroom: name})
	}
	return out, nil
}

// mcpApplier adapts the apply core (K2) to the tools port and keeps the project
// directory inside the directory the server was started in: the orchestrator
// chooses the argument, so a path outside the project is refused.
type mcpApplier struct {
	inner Applier
	root  string
	// broad is true when root is the filesystem root or the user's home: a
	// containment check against it protects nothing, so only root itself is
	// accepted as a project directory.
	broad bool
}

func (a mcpApplier) Apply(ctx context.Context, in tools.ApplyRequest) (tools.ApplyOutcome, error) {
	dir, err := a.projectDir(in.Dir)
	if err != nil {
		return tools.ApplyOutcome{}, err
	}
	r, err := a.inner.Apply(ctx, ApplyInput{Harness: in.Harness, RequestedModel: in.Model, Effort: in.Effort, AgentType: in.AgentType, TierHint: in.Tier, ProjectDir: dir})
	if err != nil {
		var bad *ApplyInputError
		if errors.As(err, &bad) {
			return tools.ApplyOutcome{}, errors.New(bad.Msg)
		}
		return tools.ApplyOutcome{}, errors.New("the decision could not be computed")
	}
	return tools.ApplyOutcome{Harness: r.Harness, RequestedModel: r.RequestedModel, SelectedModel: r.SelectedModel, Tier: r.Tier,
		Effort: r.Effort, Verdict: r.Verdict, Reason: r.Reason, Applied: r.Applied, Note: r.Note}, nil
}

// projectDir returns the directory whose policy applies. Empty means the
// server's own directory. A directory that does not exist, is not a directory,
// or resolves outside the server's directory is an error that names no path.
func (a mcpApplier) projectDir(dir string) (string, error) {
	if strings.TrimSpace(dir) == "" {
		return a.root, nil
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(a.root, dir)
	}
	if a.broad && resolvedDir(dir) != a.root {
		return "", errors.New("dir must be the directory the server was started in")
	}
	real, err := filepath.EvalSymlinks(filepath.Clean(dir))
	if err != nil {
		return "", errors.New("dir does not exist")
	}
	if fi, err := os.Stat(real); err != nil || !fi.IsDir() {
		return "", errors.New("dir is not a directory")
	}
	rel, err := filepath.Rel(a.root, real)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("dir must be inside the directory the server was started in")
	}
	return real, nil
}

// broadRoot reports whether dir is a place a containment check cannot protect:
// the filesystem root, or the user's home directory.
func broadRoot(dir string) bool {
	if dir == filepath.Dir(dir) {
		return true
	}
	home, err := os.UserHomeDir()
	return err == nil && resolvedDir(home) == dir
}

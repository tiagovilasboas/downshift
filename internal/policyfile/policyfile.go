// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package policyfile reads an optional per-project tier floor. A floor only
// raises the tier a spawn is routed to; it never lowers one. A missing file is
// a no-op. An invalid file is reported to the caller, which must fail open
// (route as if there were no file) and may surface the error in diagnostics.
package policyfile

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/core"
)

const (
	// RelPath is the policy file location relative to a project directory.
	RelPath = ".downshift/policy.json"
	// MaxDepth bounds the upward search from the starting directory.
	MaxDepth = 8
	// maxBytes bounds the file read; a policy is a few lines.
	maxBytes = 64 << 10
	version  = 1
)

// Policy is the on-disk shape. All fields are optional.
//
//	{
//	  "version": 1,
//	  "min_tier": "small",
//	  "min_tier_by_class": {"COMPLEX": "frontier"},
//	  "min_tier_by_risk":  {"payment": "frontier"}
//	}
type Policy struct {
	Version        int               `json:"version"`
	MinTier        string            `json:"min_tier,omitempty"`
	MinTierByClass map[string]string `json:"min_tier_by_class,omitempty"`
	MinTierByRisk  map[string]string `json:"min_tier_by_risk,omitempty"`
}

var classNames = map[string]bool{"TRIVIAL": true, "SIMPLE": true, "MEDIUM": true, "COMPLEX": true}

// Find returns the nearest policy file at or above startDir, searching at most
// MaxDepth directories. It reports ok=false when none exists.
func Find(startDir string) (string, bool) {
	dir := startDir
	if dir == "" {
		return "", false
	}
	for i := 0; i <= MaxDepth; i++ {
		p := filepath.Join(dir, filepath.FromSlash(RelPath))
		if fi, err := os.Lstat(p); err == nil && fi.Mode().IsRegular() {
			return p, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", false
}

// LoadNearest finds and parses the nearest policy. It returns (nil, "", nil)
// when no file exists. When a file exists but cannot be used, it returns the
// path and an error; the caller routes without a floor.
func LoadNearest(startDir string) (*Policy, string, error) {
	path, ok := Find(startDir)
	if !ok {
		return nil, "", nil
	}
	p, err := Load(path)
	if err != nil {
		return nil, path, err
	}
	return p, path, nil
}

// Load reads and validates one policy file. It refuses anything that is not a
// regular file and anything larger than the size bound.
func Load(path string) (*Policy, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, errors.New("policy file is not a regular file")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxBytes {
		return nil, errors.New("policy file too large")
	}
	var p Policy
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return nil, fmt.Errorf("policy file: %w", err)
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return &p, nil
}

// Validate rejects unknown versions, tier names, classes and risk categories,
// so a typo cannot silently disable a floor.
func (p *Policy) Validate() error {
	if p.Version != version {
		return fmt.Errorf("policy file: version must be %d", version)
	}
	if p.MinTier != "" {
		if _, ok := ParseTier(p.MinTier); !ok {
			return fmt.Errorf("policy file: unknown min_tier %q", p.MinTier)
		}
	}
	for class, tier := range p.MinTierByClass {
		if !classNames[class] {
			return fmt.Errorf("policy file: unknown class %q", class)
		}
		if _, ok := ParseTier(tier); !ok {
			return fmt.Errorf("policy file: unknown tier %q for class %s", tier, class)
		}
	}
	known := core.RiskCategoryNames()
	for risk, tier := range p.MinTierByRisk {
		if !known[risk] {
			return fmt.Errorf("policy file: unknown risk category %q", risk)
		}
		if _, ok := ParseTier(tier); !ok {
			return fmt.Errorf("policy file: unknown tier %q for risk %s", tier, risk)
		}
	}
	return nil
}

// ParseTier maps a tier name to core.Tier.
func ParseTier(s string) (core.Tier, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "small":
		return core.TierSmall, true
	case "mid":
		return core.TierMid, true
	case "frontier":
		return core.TierFrontier, true
	}
	return 0, false
}

// Floor returns the highest tier required for the given classifier class
// (TRIVIAL, SIMPLE, MEDIUM, COMPLEX) and risk categories. ok is false when no
// rule applies. A nil policy applies nothing.
func (p *Policy) Floor(class string, risks []string) (core.Tier, bool) {
	if p == nil {
		return 0, false
	}
	var best core.Tier
	found := false
	consider := func(name string) {
		if t, ok := ParseTier(name); ok && (!found || t > best) {
			best, found = t, true
		}
	}
	if p.MinTier != "" {
		consider(p.MinTier)
	}
	if name, ok := p.MinTierByClass[class]; ok {
		consider(name)
	}
	for _, r := range risks {
		if name, ok := p.MinTierByRisk[r]; ok {
			consider(name)
		}
	}
	return best, found
}

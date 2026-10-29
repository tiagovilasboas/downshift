// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Tier floor: an optional per-project minimum tier. A floor only raises the
// tier a spawn is routed to; it never lowers one. A missing file is a no-op. An
// invalid file is reported to the caller, which must fail open (route as if
// there were no file). The file is the same policy.json that holds the routing
// section, so FloorPolicy accepts and ignores that key.
//
// Downshift by Tiago Vilas Boas
// https://github.com/tiagovilasboas/downshift

package policy

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/core"
)

const (
	// FloorRelPath is the floor file location relative to a project directory.
	FloorRelPath = ".downshift/policy.json"
	// FloorMaxDepth bounds the upward search from the starting directory.
	FloorMaxDepth = 8
	// FloorMaxBytes bounds the file read; a floor is a few lines.
	FloorMaxBytes = maxBytes
	floorVersion  = 1
)

// FloorPolicy is the on-disk shape of the tier floor. All fields are optional.
//
//	{
//	  "version": 1,
//	  "min_tier": "small",
//	  "min_tier_by_class": {"COMPLEX": "frontier"},
//	  "min_tier_by_risk":  {"payment": "frontier"}
//	}
type FloorPolicy struct {
	Version        int               `json:"version"`
	MinTier        string            `json:"min_tier,omitempty"`
	MinTierByClass map[string]string `json:"min_tier_by_class,omitempty"`
	MinTierByRisk  map[string]string `json:"min_tier_by_risk,omitempty"`
	// Mode is the routing mode (economy or standard). The routing loader reads
	// and validates it; it is accepted here so the strict decoder does not
	// refuse a file that declares one.
	Mode string `json:"mode,omitempty"`
	// Routing is the routing policy section. internal/routing/policy reads and
	// validates it; it is accepted here so that a routing section in the same
	// file does not refuse the tier floor (the decoder rejects unknown keys).
	Routing json.RawMessage `json:"routing,omitempty"`
}

var floorClassNames = map[string]bool{"TRIVIAL": true, "SIMPLE": true, "MEDIUM": true, "COMPLEX": true}

// FindFloor returns the nearest policy file at or above startDir, searching at most
// FloorMaxDepth directories. ok is false when none exists. Only a missing entry
// (ENOENT) counts as absent: any other failure to look, such as a directory
// without search permission, is returned so the caller can refuse instead of
// routing as if no file were there.
func FindFloor(startDir string) (path string, ok bool, err error) {
	dir := startDir
	if dir == "" {
		return "", false, nil
	}
	for i := 0; i <= FloorMaxDepth; i++ {
		p := filepath.Join(dir, filepath.FromSlash(FloorRelPath))
		fi, statErr := os.Lstat(p)
		switch {
		case statErr == nil && fi.Mode().IsRegular():
			return p, true, nil
		case statErr != nil && !errors.Is(statErr, fs.ErrNotExist):
			return "", false, statErr
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", false, nil
}

// LoadNearestFloor finds and parses the nearest policy. It returns (nil, "", nil)
// when no file exists. When a file exists but cannot be used, or cannot be
// looked up, it returns the path (empty when the lookup failed) and an error;
// the caller routes without a floor.
func LoadNearestFloor(startDir string) (*FloorPolicy, string, error) {
	path, ok, err := FindFloor(startDir)
	if err != nil {
		return nil, "", err
	}
	if !ok {
		return nil, "", nil
	}
	p, err := LoadFloor(path)
	if err != nil {
		return nil, path, err
	}
	return p, path, nil
}

// LoadFloor reads and validates one policy file. It refuses anything that is not a
// regular file and anything larger than the size bound.
func LoadFloor(path string) (*FloorPolicy, error) {
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
	var p FloorPolicy
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
func (p *FloorPolicy) Validate() error {
	if p.Version != floorVersion {
		return fmt.Errorf("policy file: version must be %d", floorVersion)
	}
	if p.Mode != "" && !Mode(p.Mode).Valid() {
		return fmt.Errorf("policy file: unknown mode %q", p.Mode)
	}
	if p.MinTier != "" {
		if _, ok := ParseFloorTier(p.MinTier); !ok {
			return fmt.Errorf("policy file: unknown min_tier %q", p.MinTier)
		}
	}
	for class, tier := range p.MinTierByClass {
		if !floorClassNames[class] {
			return fmt.Errorf("policy file: unknown class %q", class)
		}
		if _, ok := ParseFloorTier(tier); !ok {
			return fmt.Errorf("policy file: unknown tier %q for class %s", tier, class)
		}
	}
	known := core.RiskCategoryNames()
	for risk, tier := range p.MinTierByRisk {
		if !known[risk] {
			return fmt.Errorf("policy file: unknown risk category %q", risk)
		}
		if _, ok := ParseFloorTier(tier); !ok {
			return fmt.Errorf("policy file: unknown tier %q for risk %s", tier, risk)
		}
	}
	return nil
}

// ParseFloorTier maps a tier name to core.Tier.
func ParseFloorTier(s string) (core.Tier, bool) {
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
func (p *FloorPolicy) Floor(class string, risks []string) (core.Tier, bool) {
	if p == nil {
		return 0, false
	}
	var best core.Tier
	found := false
	consider := func(name string) {
		if t, ok := ParseFloorTier(name); ok && (!found || t > best) {
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

// ApplyFloor raises d to the project's tier floor when a valid policy file
// applies. Any problem (no file, invalid file, no usable directory) leaves the
// decision untouched, so callers fail open. cwd is the hook payload's working
// directory; when empty the process directory is used. currentModel is the
// model in the payload, as passed to core.Route.
func ApplyFloor(d *core.Decision, cwd, currentModel string, res core.Resolver) {
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	p, _, err := LoadNearestFloor(cwd)
	if err != nil || p == nil {
		return
	}
	if tier, ok := p.Floor(d.Complexity.String(), d.Risk); ok {
		d.RaiseTier(tier, currentModel, res)
	}
}

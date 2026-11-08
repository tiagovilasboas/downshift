// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/tiagovilasboas/downshift/internal/routing/policy"
)

// PolicyInitDocument is the bare file `downshift policy init --bare` creates,
// and what init creates when no harness has a small-tier model it could write.
// It holds the version and nothing else: a file without a routing key has no
// routing policy and sets no tier floor, so it changes no routing until the
// user adds a section.
const PolicyInitDocument = "{\n  \"version\": 1\n}\n"

// ExploreProfile is the name of the profile `policy init` offers. A profile is
// matched to a spawn by the agent type's lower-cased name, so it applies to the
// harness's read-only exploration agent type (Claude Code's `Explore`). A
// profile is declared policy: it bounds the orchestrator's choice on every spawn
// of that type (precedence 2, above a model named on the call); only a model the
// agent definition pins stays. A model the session list does not hold is never
// written, and a harness with no list gets no profile.
const ExploreProfile = "explore"

// exploreDescription is the description written into the profile. It is the
// text `policy apply` would put on a compiled agent definition.
const exploreDescription = "Read-only exploration: search, read and summarise. Runs on the small tier; delete this profile to let the orchestrator choose."

// SmallTarget is the small-tier model one harness would write today, as the
// CLI resolved it from the detected model list.
type SmallTarget struct {
	Harness string
	// Model is the id the list ranks as the small tier; empty when the harness
	// has no detected list, so nothing can be promised for it.
	Model string
	// Native is the name the harness's spawn schema accepts for Model when it
	// differs from the id (Claude Code's family alias); empty otherwise.
	Native string
	// Source names where the list came from (payload, discovery, file or the
	// catalog fallback), for the note.
	Source string
}

// policyInitTarget labels the plan in the output. The file belongs to the
// project, not to a harness.
var policyInitTarget = Target{Name: "policy"}

// PlanPolicyInit plans the project policy file under o.ProjectDir. It never
// writes. ok is false when no project directory is set, which is how setup
// skips the step in tests and in the paths that must not touch a project.
// With resolved SmallTargets and BarePolicy false, the file declares one
// ExploreProfile per harness that has a small-tier model, and the plan's notes
// say which model each one resolves to today and which harnesses were left
// out and why.
func PlanPolicyInit(o Options) (plan Plan, ok bool) {
	if o.ProjectDir == "" {
		return Plan{}, false
	}
	path := policy.ProjectPath(o.ProjectDir)
	// From the directory that holds the state directory (usually HOME) the
	// project file is the user policy (<state dir>/policy.json). Writing it there
	// would make it every project's policy through the nearest-file search, so
	// setup offers nothing and says why. `policy init` still writes it on request.
	if filepath.Clean(path) == filepath.Clean(filepath.Join(o.StatePath(), policy.FileName)) {
		return Plan{Target: policyInitTarget, Path: path,
			Skip: Display(o.Home, path) + " is your user policy file, not a project's; run `downshift setup` inside a project to create that project's policy"}, true
	}
	doc, notes := PolicyInitDocumentFor(o.SmallTargets, o.BarePolicy)
	plan = Plan{Target: policyInitTarget, Path: path, After: []byte(doc), Notes: notes}
	_, err := os.Lstat(path)
	switch {
	case err == nil:
		plan.Skip = Display(o.Home, path) + " already exists, so it is left untouched"
		plan.Notes = nil
	case errors.Is(err, fs.ErrNotExist):
	default:
		plan.Err = err
	}
	return plan, true
}

// PolicyInitDocumentFor builds the policy file for the resolved targets and the
// notes that explain it. Targets with no model add a note and no profile. With
// bare set, or no target resolved, the document is PolicyInitDocument.
func PolicyInitDocumentFor(targets []SmallTarget, bare bool) (doc string, notes []string) {
	sorted := append([]SmallTarget(nil), targets...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Harness < sorted[j].Harness })
	type profileDoc struct {
		Name        string `json:"name"`
		Tier        string `json:"tier"`
		Harness     string `json:"harness"`
		Description string `json:"description"`
	}
	var profiles []profileDoc
	for _, t := range sorted {
		if t.Model == "" {
			notes = append(notes, fmt.Sprintf("%s: no detected model list, so no profile is written for it; run `downshift models discover --harness %s`, then `downshift policy init` again", t.Harness, t.Harness))
			continue
		}
		if bare {
			continue
		}
		written := t.Model
		if t.Native != "" && t.Native != t.Model {
			written = fmt.Sprintf("%s (written as %q)", t.Model, t.Native)
		}
		notes = append(notes, fmt.Sprintf("%s: profile %q sends the agent type of that name (matched case-insensitively, so Claude Code's Explore) to the small tier, today %s, from the %s list. A declared profile bounds the orchestrator's choice on every such spawn, a model named on the call included; delete the profile to hand that agent type back to the orchestrator", t.Harness, ExploreProfile, written, t.Source))
		profiles = append(profiles, profileDoc{Name: ExploreProfile, Tier: "small", Harness: t.Harness, Description: exploreDescription})
	}
	if len(profiles) == 0 {
		return PolicyInitDocument, notes
	}
	out := struct {
		Version int `json:"version"`
		Routing struct {
			Profiles []profileDoc `json:"profiles"`
		} `json:"routing"`
	}{Version: 1}
	out.Routing.Profiles = profiles
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return PolicyInitDocument, notes
	}
	return string(b) + "\n", notes
}

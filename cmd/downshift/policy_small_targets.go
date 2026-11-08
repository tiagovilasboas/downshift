// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/models"
	"github.com/tiagovilasboas/downshift-ai/internal/setup"
)

// resolveSmallTargets answers, for every harness whose hook can rewrite a
// subagent's model, which small-tier model that hook would write today. It
// reads the same sources the hook does (models.Resolve: payload-less here, so
// discovery cache, session-models.json, then the catalog stand-in where the
// harness declares it) and never names a model no source listed. A harness
// with no list, or whose list ranks nothing as small, comes back with an empty
// Model so the caller can say so instead of guessing. A nil md resolves nothing.
func resolveSmallTargets(md models.Metadata) []setup.SmallTarget {
	if md == nil {
		return nil
	}
	var out []setup.SmallTarget
	for _, nc := range core.AllHarnessCaps() {
		if !nc.Caps.CanRewriteModel {
			continue
		}
		t := setup.SmallTarget{Harness: nc.Harness}
		if l, err := models.Resolve(nc.Harness, md, models.Options{}); err == nil {
			for _, m := range l.Models {
				if m.Untiered || m.Tier != core.TierSmall.String() {
					continue
				}
				t.Model, t.Source = m.ID, m.Source
				if cm, ok := md.LookupByID(nc.Harness, m.ID); ok {
					t.Native = cm.Native
				}
				break
			}
		}
		out = append(out, t)
	}
	return out
}

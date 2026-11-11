// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/console/setupwizard"
	"github.com/tiagovilasboas/downshift-ai/internal/console/shared/theme"
	"github.com/tiagovilasboas/downshift-ai/internal/setup"
)

func presetByID(ps []setupwizard.Preset) map[string]setupwizard.Preset {
	out := map[string]setupwizard.Preset{}
	for _, p := range ps {
		out[p.ID] = p
	}
	return out
}

func TestWizardPresets(t *testing.T) {
	o := setup.Options{Mcp: true, ProjectDir: t.TempDir()}
	ps := wizardPresets(wizardComponents(o))
	if ps[0].ID != "essential" {
		t.Fatalf("first preset = %q, want essential (what Enter picks)", ps[0].ID)
	}
	by := presetByID(ps)
	if got := by["essential"].Components; !reflect.DeepEqual(got, []string{"hooks", "plugin", "steering", "policy"}) {
		t.Errorf("essential = %v", got)
	}
	complete := by["complete"].Components
	for _, id := range []string{"hooks", "mcp", "plugin", "mods", "session-policy", "skills", "steering", "policy"} {
		if !strings.Contains(" "+strings.Join(complete, " ")+" ", " "+id+" ") {
			t.Errorf("complete lacks %s: %v", id, complete)
		}
	}
	for _, p := range ps {
		for _, id := range p.Components {
			if id == "prune" || id == "you-should-know" {
				t.Errorf("%s selects %s, which must stay opt-in", p.ID, id)
			}
		}
	}
	if !by["custom"].Custom || !by["complete"].AskChoice || by["essential"].AskChoice {
		t.Errorf("custom/complete/essential flags wrong: %+v", ps)
	}
}

func TestWizardPresetsKeepFlagsThatTurnedComponentsOff(t *testing.T) {
	// --no-plugin and no project directory: Essential must not bring them back.
	o := setup.Options{NoPlugin: true}
	by := presetByID(wizardPresets(wizardComponents(o)))
	if got := by["essential"].Components; !reflect.DeepEqual(got, []string{"hooks", "steering"}) {
		t.Errorf("essential = %v, want hooks and steering only", got)
	}
}

func TestSetupSummary(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var b bytes.Buffer
	writeSetupSummary(&b, theme.New(), setupwizard.Selection{
		Harnesses: []string{"claude-code"}, Components: []string{"hooks", "plugin"}, Preset: "essential",
	})
	out := b.String()
	for _, want := range []string{"✓ ", "Downshift is set up", "Harnesses", "claude-code", "Profile",
		"Essential", "Components", "hooks, plugin", "downshift setup claude-code", "1. Open a new session", "downshift doctor", "downshift stats"} {
		if !strings.Contains(out, want) {
			t.Errorf("summary lacks %q:\n%s", want, out)
		}
	}
}

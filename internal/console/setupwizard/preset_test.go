// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setupwizard

import (
	"reflect"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func presetOptions() Options {
	o := sampleOptions(nil)
	o.Choice = &Choice{ID: "workers", Title: "Worker tools", Items: []ChoiceItem{
		{ID: "read", Label: "read (recommended)"}, {ID: "off", Label: "off"},
	}}
	o.Presets = []Preset{
		{ID: "essential", Label: "Essential (recommended)", Hint: "hooks only", Components: []string{"hooks", "prune-binaries"}},
		{ID: "complete", Label: "Complete", Hint: "everything safe", Components: []string{"hooks", "mcp"}, AskChoice: true},
		{ID: "custom", Label: "Custom", Hint: "pick each one", Custom: true},
	}
	return o
}

// review runs the pending preview command, as deliverPreview does after an Enter.
func review(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	if m.step != stepReview {
		t.Fatalf("step = %d, want review", m.step)
	}
	for _, c := range flatten(cmd) {
		if msg, ok := c().(previewMsg); ok {
			m, _ = m.Update(msg)
		}
	}
	return m
}

func TestPresetEssentialSkipsTheListAndTheQuestion(t *testing.T) {
	m := press(newModel(t, presetOptions()), enter)
	if m.step != stepPreset || m.ordinal() != 2 {
		t.Fatalf("step=%d ordinal=%d, want the preset step second", m.step, m.ordinal())
	}
	m, cmd := m.Update(enter) // Essential is first
	m = review(t, m, cmd)
	sel := m.Selection()
	// prune-binaries is destructive: a preset never selects it.
	if !reflect.DeepEqual(sel.Components, []string{"hooks"}) || sel.Preset != "essential" || sel.Choice != "" {
		t.Fatalf("selection = %+v", sel)
	}
	if m.ordinal() != 3 || m.total() != 4 {
		t.Errorf("review is step %d/%d, want 3/4", m.ordinal(), m.total())
	}
}

func TestPresetCompleteAsksTheQuestion(t *testing.T) {
	m := press(newModel(t, presetOptions()), enter, down, enter)
	if m.step != stepChoice {
		t.Fatalf("step = %d, want the question", m.step)
	}
	m, cmd := m.Update(enter)
	m = review(t, m, cmd)
	sel := m.Selection()
	if !reflect.DeepEqual(sel.Components, []string{"hooks", "mcp"}) || sel.Choice != "read" || sel.Preset != "complete" {
		t.Fatalf("selection = %+v", sel)
	}
}

func TestPresetCustomOpensTheList(t *testing.T) {
	m := press(newModel(t, presetOptions()), enter, down, down, enter)
	if m.step != stepComponents || m.ordinal() != 3 || m.total() != 6 {
		t.Fatalf("step=%d at %d/%d, want the component list at 3/6", m.step, m.ordinal(), m.total())
	}
	if m = press(m, enter); m.step != stepChoice {
		t.Fatalf("step = %d, want the question after the list", m.step)
	}
}

func TestPresetBackGoesThroughThePath(t *testing.T) {
	m := press(newModel(t, presetOptions()), enter)
	m, _ = m.Update(enter) // Essential, now on the review
	if m = press(m, esc); m.step != stepPreset {
		t.Fatalf("esc from the review: step %d, want the presets", m.step)
	}
	if !strings.Contains(m.View(), "› Essential") && !strings.Contains(m.View(), "> Essential") {
		t.Errorf("the cursor should stay on the preset picked:\n%s", m.View())
	}
	if m = press(m, esc); m.step != stepHarnesses {
		t.Fatalf("esc from the presets: step %d, want the harnesses", m.step)
	}
	m = press(m, enter, down, down, enter, enter) // Custom, list, question
	if m = press(m, esc); m.step != stepComponents {
		t.Fatalf("esc from the question on the custom path: step %d, want the list", m.step)
	}
	if m = press(m, esc); m.step != stepPreset {
		t.Fatalf("esc from the list: step %d, want the presets", m.step)
	}
}

func TestDoneLeavesOneLineInTheScrollback(t *testing.T) {
	m := press(newModel(t, presetOptions()), enter)
	m, cmd := m.Update(enter)
	m = review(t, m, cmd)
	m = press(m, enter, keyY)
	if !m.Done() {
		t.Fatal("not done after yes")
	}
	got := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(m.View(), "")
	if got != "✓ Setting up claude-code, codex · Essential (recommended): hooks\n" {
		t.Errorf("done line = %q", got)
	}
	if c := press(newModel(t, presetOptions()), ctrlC); c.View() != "" {
		t.Errorf("a cancelled wizard leaves %q, want nothing", c.View())
	}
}

func TestGoldenPresetStep(t *testing.T) {
	m := newModel(t, presetOptions())
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	golden(t, "w80_2_presets", press(m, enter).View())
}

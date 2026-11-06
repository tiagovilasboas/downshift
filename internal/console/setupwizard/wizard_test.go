// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setupwizard

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/tiagovilasboas/downshift/internal/console/shared/diffview"
	"github.com/tiagovilasboas/downshift/internal/console/shared/theme"
)

func sampleOptions(preview func(Selection) ([]diffview.File, error)) Options {
	if preview == nil {
		preview = func(Selection) ([]diffview.File, error) {
			return []diffview.File{{Path: "~/.claude/settings.json", Lines: []string{"@@ -1 +1 @@", "-a", "+b"}}}, nil
		}
	}
	return Options{
		Harnesses: []Harness{
			{ID: "claude-code", Label: "Claude Code", Detected: true},
			{ID: "codex", Label: "Codex", Detected: true, Note: "config found"},
			{ID: "cursor", Label: "Cursor"},
		},
		Components: []Component{
			{ID: "hooks", Label: "Hooks", Hint: "route subagent models", Default: true},
			{ID: "mcp", Label: "MCP server", Hint: "register the MCP server", Default: true},
			{ID: "prune-binaries", Label: "Remove other binaries", Hint: "deletes other downshift binaries on PATH", Default: true, Destructive: true},
		},
		Preview: preview,
	}
}

func newModel(t *testing.T, o Options) Model {
	t.Setenv("NO_COLOR", "1")
	return New(theme.New(), o)
}

func press(m Model, keys ...tea.KeyPressMsg) Model {
	for _, k := range keys {
		m, _ = m.Update(k)
	}
	return m
}

var (
	enter     = tea.KeyPressMsg{Code: tea.KeyEnter}
	esc       = tea.KeyPressMsg{Code: tea.KeyEscape}
	backspace = tea.KeyPressMsg{Code: tea.KeyBackspace}
	ctrlC     = tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	space     = tea.KeyPressMsg{Code: ' ', Text: " "}
	down      = tea.KeyPressMsg{Code: tea.KeyDown}
	keyN      = tea.KeyPressMsg{Code: 'n', Text: "n"}
	keyY      = tea.KeyPressMsg{Code: 'y', Text: "y"}
)

// deliverPreview presses enter on the components step and feeds the preview result back.
func deliverPreview(t *testing.T, m Model) Model {
	t.Helper()
	m, cmd := m.Update(enter)
	if m.step != stepReview || !m.loading {
		t.Fatalf("step=%d loading=%v, want review while loading", m.step, m.loading)
	}
	for _, c := range flatten(cmd) {
		if msg, ok := c().(previewMsg); ok {
			m, _ = m.Update(msg)
		}
	}
	return m
}

func flatten(cmd tea.Cmd) []tea.Cmd {
	if cmd == nil {
		return nil
	}
	if b, ok := cmd().(tea.BatchMsg); ok {
		return b
	}
	return []tea.Cmd{cmd}
}

func TestHappyPath(t *testing.T) {
	m := newModel(t, sampleOptions(nil))
	if got := m.Selection().Harnesses; !reflect.DeepEqual(got, []string{"claude-code", "codex"}) {
		t.Fatalf("detected harnesses preselected: %v", got)
	}
	m = press(m, enter)
	if m.step != stepComponents {
		t.Fatalf("step = %d, want components", m.step)
	}
	if got := m.Selection().Components; !reflect.DeepEqual(got, []string{"hooks", "mcp"}) {
		t.Fatalf("destructive component must start unselected: %v", got)
	}
	m = deliverPreview(t, m)
	if m.loading || m.err != nil || len(m.files) != 1 {
		t.Fatalf("preview not applied: loading=%v err=%v files=%d", m.loading, m.err, len(m.files))
	}
	m = press(m, enter)
	if m.step != stepConfirm || m.Done() {
		t.Fatalf("step=%d done=%v, want confirm", m.step, m.Done())
	}
	m = press(m, enter) // non-destructive: Yes is focused
	if !m.Done() || m.Cancelled() {
		t.Errorf("done=%v cancelled=%v, want done", m.Done(), m.Cancelled())
	}
}

func TestBackNavigation(t *testing.T) {
	m := newModel(t, sampleOptions(nil))
	m = press(m, enter)
	m = deliverPreview(t, m)
	m = press(m, enter)
	for _, want := range []step{stepReview, stepComponents, stepHarnesses} {
		key := esc
		if want == stepComponents {
			key = backspace
		}
		m = press(m, key)
		if m.step != want {
			t.Fatalf("back to step %d, got %d", want, m.step)
		}
	}
	if m.Cancelled() {
		t.Error("going back must not cancel")
	}
	m = press(m, backspace)
	if m.step != stepHarnesses || m.Cancelled() {
		t.Error("backspace on the first step does nothing")
	}
}

func TestCancel(t *testing.T) {
	if m := press(newModel(t, sampleOptions(nil)), esc); !m.Cancelled() || m.Done() {
		t.Error("esc on step 1 must cancel")
	}
	m := press(newModel(t, sampleOptions(nil)), enter)
	if m = press(m, ctrlC); !m.Cancelled() {
		t.Error("ctrl+c must cancel on any step")
	}
}

func TestEmptyHarnessSelectionBlocks(t *testing.T) {
	m := newModel(t, sampleOptions(nil))
	m = press(m, keyN, enter)
	if m.step != stepHarnesses {
		t.Fatalf("empty selection advanced to step %d", m.step)
	}
	if !strings.Contains(m.View(), "Select at least one harness") {
		t.Errorf("no inline message:\n%s", m.View())
	}
	m = press(m, space, enter)
	if m.step != stepComponents {
		t.Errorf("step = %d after selecting one, want components", m.step)
	}
}

func TestPreviewError(t *testing.T) {
	m := newModel(t, sampleOptions(func(Selection) ([]diffview.File, error) { return nil, errors.New("boom") }))
	m = deliverPreview(t, press(m, enter))
	if m.err == nil || !strings.Contains(m.View(), "preview failed: boom") {
		t.Fatalf("error not shown:\n%s", m.View())
	}
	if m = press(m, enter); m.step != stepReview {
		t.Error("enter must not advance past a failed preview")
	}
	if m = press(m, esc); m.step != stepComponents {
		t.Errorf("esc after an error goes back, step=%d", m.step)
	}
}

func TestStalePreviewIsDropped(t *testing.T) {
	m := newModel(t, sampleOptions(nil))
	m, _ = press(m, enter).Update(enter)
	stale := previewMsg{gen: m.gen}
	m = press(m, esc)
	m, _ = m.Update(stale)
	if m.step != stepComponents || m.files != nil {
		t.Error("a preview from a left review must be ignored")
	}
}

func TestEmptyPreviewSaysNothingToChange(t *testing.T) {
	m := newModel(t, sampleOptions(func(Selection) ([]diffview.File, error) { return nil, nil }))
	m = deliverPreview(t, press(m, enter))
	if !strings.Contains(m.View(), "Nothing to change.") {
		t.Errorf("missing empty message:\n%s", m.View())
	}
}

func TestDestructiveConfirmDefaultsToNo(t *testing.T) {
	m := newModel(t, sampleOptions(nil))
	m = press(m, enter, down, down, space) // select the destructive component
	m = deliverPreview(t, m)
	m = press(m, enter)
	if m.step != stepConfirm || !strings.Contains(m.View(), "destructive: Remove other binaries") {
		t.Fatalf("destructive item not listed:\n%s", m.View())
	}
	if m = press(m, enter); m.Done() || m.step != stepReview {
		t.Errorf("enter on the default (No) must not apply: done=%v step=%d", m.Done(), m.step)
	}
	m = press(m, enter, keyY)
	if !m.Done() {
		t.Error("explicit y must apply")
	}
}

func TestEquivalentCommand(t *testing.T) {
	toArgs := func(s Selection) []string { return []string{"--harness=" + strings.Join(s.Harnesses, ",")} }
	got := EquivalentCommand(Selection{Harnesses: []string{"codex"}}, toArgs)
	if got != "downshift setup --harness=codex" {
		t.Errorf("got %q", got)
	}
	if got := EquivalentCommand(Selection{}, func(Selection) []string { return nil }); got != "downshift setup" {
		t.Errorf("no args: %q", got)
	}
}

func choiceOptions() Options {
	o := sampleOptions(nil)
	o.Choice = &Choice{ID: "workers", Title: "Worker tools", Items: []ChoiceItem{
		{ID: "read", Label: "read (recommended)"}, {ID: "off", Label: "off"}, {ID: "write", Label: "write"},
	}}
	return o
}

func TestChoiceStepAsksAfterComponentsAndDefaultsToFirst(t *testing.T) {
	m := press(newModel(t, choiceOptions()), enter, enter)
	if m.step != stepChoice {
		t.Fatalf("step = %v, want the choice step", m.step)
	}
	v := m.View()
	if !strings.Contains(v, "step 3/5") || !strings.Contains(v, "Worker tools") || !strings.Contains(v, "read (recommended)") {
		t.Errorf("choice view:\n%s", v)
	}
	m = press(m, enter)
	if m.step != stepReview || m.Selection().Choice != "read" {
		t.Errorf("step %v choice %q, want review and read", m.step, m.Selection().Choice)
	}
	if m.ordinal() != 4 {
		t.Errorf("review ordinal = %d, want 4", m.ordinal())
	}
}

func TestChoiceStepPicksWriteOnlyWhenChosenAndBackReturns(t *testing.T) {
	m := press(newModel(t, choiceOptions()), enter, enter, down, down, enter)
	if got := m.Selection().Choice; got != "write" {
		t.Errorf("choice = %q, want write", got)
	}
	m = press(m, esc)
	if m.step != stepChoice {
		t.Errorf("esc from review: step %v, want the choice step", m.step)
	}
	m = press(m, esc)
	if m.step != stepComponents {
		t.Errorf("esc from choice: step %v, want components", m.step)
	}
}

func TestNoChoiceKeepsFourSteps(t *testing.T) {
	m := press(newModel(t, sampleOptions(nil)), enter, enter)
	if m.step != stepReview || m.total() != 4 || m.Selection().Choice != "" {
		t.Errorf("step %v total %d choice %q", m.step, m.total(), m.Selection().Choice)
	}
}

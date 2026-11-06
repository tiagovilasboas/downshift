// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"

	"github.com/tiagovilasboas/downshift/internal/console/setupwizard"
	"github.com/tiagovilasboas/downshift/internal/console/shared/diffview"
	"github.com/tiagovilasboas/downshift/internal/setup"
)

// The wizard is a seam so tests can run the flow without a terminal.
var (
	setupWizardRun = setupwizard.Run
	setupHasTTY    = isInteractiveTerminal
)

// setupDirs are the directories the wizard may offer to act on: the project
// that gets a policy file and the places searched for old downshift copies.
type setupDirs struct {
	project string
	bins    []string
}

// setupTUI runs the interactive setup: pick harnesses and components, review
// the dry-run diff, confirm, then apply through the same path as --yes. With
// no terminal on the input it falls back to the plain flow, which shows the
// diff and asks on that input (nothing is written without a yes).
func setupTUI(o setup.Options) int {
	if !setupHasTTY(o.In) {
		return setupRun(o)
	}
	if _, code := setup.Select(o); code != 0 {
		return code
	}
	dirs := wizardDirs(o)
	wo := setupwizard.Options{
		Harnesses:  wizardHarnesses(o),
		Components: wizardComponents(o),
		Preview:    func(s setupwizard.Selection) ([]diffview.File, error) { return previewSetup(o, dirs, s) },
	}
	if o.Workers == "" { // a --workers flag already answered the question
		wo.Choice = workersChoice()
	}
	sel, confirmed, err := setupWizardRun(wo, o.In, o.Out)
	if err != nil {
		fmt.Fprintf(o.Err, "setup: the wizard failed: %v\n", err)
		return 1
	}
	if !confirmed {
		fmt.Fprintln(o.Out, "Setup cancelled. Nothing was written.")
		return 0
	}
	run := selectedSetup(o, dirs, sel)
	run.Yes = true
	code := setupRun(run)
	if code == 0 {
		renderSetupDone(run, sel)
	}
	return code
}

// wizardDirs resolves what the policy and binary-cleanup components act on.
func wizardDirs(o setup.Options) setupDirs {
	d := setupDirs{project: o.ProjectDir, bins: o.BinDirs}
	if d.project == "" {
		d.project, _ = os.Getwd()
	}
	if d.bins == nil {
		d.bins = setup.DefaultBinDirs(o.Home)
	}
	return d
}

// selectedSetup is the options to run for a selection: the pure mapping plus
// the directories the policy and cleanup components switch on or off.
func selectedSetup(base setup.Options, d setupDirs, sel setupwizard.Selection) setup.Options {
	o, f := selectionOptions(base, sel)
	o.ProjectDir, o.BinDirs = "", nil
	if !f.noPolicy {
		o.ProjectDir = d.project
	}
	if !f.keepBinaries {
		o.BinDirs = d.bins
	}
	return o
}

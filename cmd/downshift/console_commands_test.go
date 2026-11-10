// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/clispec"
)

// consoleSkip are help topics the console does not list: hook plumbing the
// harness calls, a removed command, help itself, and the hidden docs-manifest
// a release step runs.
var consoleSkip = map[string]bool{"dci": true, "help": true, "honor": true, "usage": true, "end": true, "docs-manifest": true}

func TestConsoleListsEveryCommand(t *testing.T) {
	listed := map[string]bool{}
	for _, s := range consoleCommands {
		listed[s.Args[0]] = true
		if len(s.Args) > 1 {
			listed[s.Args[0]+" "+s.Args[1]] = true
		}
	}
	for _, topics := range []map[string]string{dailyHelp, gatewayHelp, experimentalHelp, maintainerHelp, modelsHelp, mcpHelp, playbookHelp, policyHelp, routeHelp, retrieveHelp, discoverHelp} {
		for name := range topics {
			if !consoleSkip[name] && !listed[name] {
				t.Errorf("help topic %q has no console entry in consoleCommands", name)
			}
		}
	}
}

// The console groups what clispec marks Experimental with the experimental or
// maintainer entries, not with the daily ones.
func TestConsoleGroupsExperimentalCommands(t *testing.T) {
	experimental := map[string]bool{}
	for _, n := range clispec.ExperimentalNames() {
		experimental[n] = true
	}
	for _, c := range consoleCommands {
		if !experimental[c.Args[0]] {
			continue
		}
		if c.Group != "experimental" && c.Group != "maintainer" {
			t.Errorf("console entry %v is experimental in clispec but sits in group %q", c.Args, c.Group)
		}
	}
}

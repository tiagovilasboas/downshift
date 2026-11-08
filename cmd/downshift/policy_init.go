// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/catalog"
	"github.com/tiagovilasboas/downshift-ai/internal/models"
	"github.com/tiagovilasboas/downshift-ai/internal/setup"
)

const policyInitUsage = `usage: downshift policy init [--dry-run] [--yes] [--bare]

Create .downshift/policy.json in the current directory. For each harness whose
hook can rewrite a subagent's model and has a detected model list, the file
declares an "explore" profile that sends that agent type to the small tier; the
plan names the model each one resolves to today. A harness with no list gets no
profile and a note saying what to run. An existing file is left untouched.
Nothing is written without a yes; --dry-run shows the file that would be created.
  --dry-run   show what would be written, write nothing
  --yes       do not ask before writing
  --bare      write only the version, no routing section
`

// runPolicyInit is 'downshift policy init'. It shares the plan, confirm and
// write steps with 'downshift setup', so both show and write the file the same way.
func runPolicyInit(args []string, in io.Reader, stdout, stderr io.Writer) int {
	o := setup.Options{In: in, Out: stdout, Err: stderr, Now: time.Now}
	for _, a := range args {
		switch a {
		case "--dry-run":
			o.DryRun = true
		case "--yes", "-y":
			o.Yes = true
		case "--bare":
			o.BarePolicy = true
		case "-h", "--help":
			fmt.Fprint(stdout, policyInitUsage)
			return 0
		default:
			fmt.Fprintf(stderr, "policy init: unknown argument %q\n%s", a, policyInitUsage)
			return 2
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(stderr, "policy init: cannot find the home directory: %v\n", err)
		return 1
	}
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "policy init: %v\n", err)
		return 1
	}
	o.Home = home
	o.ProjectDir = cwd
	if md, ok := any(catalog.Load()).(models.Metadata); ok {
		o.SmallTargets = resolveSmallTargets(md)
	}
	plan, _ := setup.PlanPolicyInit(o)
	failed, _ := setup.ApplyPlans(o, []setup.Plan{plan})
	if failed {
		return 1
	}
	return 0
}

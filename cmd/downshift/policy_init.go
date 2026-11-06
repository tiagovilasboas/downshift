// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/tiagovilasboas/downshift/internal/setup"
)

const policyInitUsage = `usage: downshift policy init [--dry-run] [--yes]

Create .downshift/policy.json in the current directory, with no routing policy.
An existing file is left untouched. Nothing is written without a yes; --dry-run
shows the file that would be created.
  --dry-run   show what would be written, write nothing
  --yes       do not ask before writing
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
	plan, _ := setup.PlanPolicyInit(o)
	failed, _ := setup.ApplyPlans(o, []setup.Plan{plan})
	if failed {
		return 1
	}
	return 0
}

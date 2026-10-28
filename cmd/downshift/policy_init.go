// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"time"

	"github.com/tiagovilasboas/downshift/internal/routing/policy"
)

// policyInitDocument is the file `downshift policy init` creates. It holds the
// version and nothing else: a file without a routing key has no routing policy
// and sets no tier floor, so the new file changes no routing until the user
// adds a section.
const policyInitDocument = "{\n  \"version\": 1\n}\n"

// policyInitTarget labels the plan in the output. The file belongs to the
// project, not to a harness.
var policyInitTarget = setupTarget{Name: "policy"}

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
	o := setupOptions{In: in, Out: stdout, Err: stderr, Now: time.Now}
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
	plan, _ := planPolicyInit(o)
	failed, _ := applyPlans(o, []setupPlan{plan})
	if failed {
		return 1
	}
	return 0
}

// planPolicyInit plans the project policy file under o.ProjectDir. It never
// writes. ok is false when no project directory is set, which is how setup
// skips the step in tests and in the paths that must not touch a project.
func planPolicyInit(o setupOptions) (plan setupPlan, ok bool) {
	if o.ProjectDir == "" {
		return setupPlan{}, false
	}
	path := policy.ProjectPath(o.ProjectDir)
	plan = setupPlan{target: policyInitTarget, path: path, after: []byte(policyInitDocument)}
	_, err := os.Lstat(path)
	switch {
	case err == nil:
		plan.skip = setupDisplay(o.Home, path) + " already exists, so it is left untouched"
	case errors.Is(err, fs.ErrNotExist):
	default:
		plan.err = err
	}
	return plan, true
}

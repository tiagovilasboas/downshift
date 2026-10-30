// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/policyfile"
	"github.com/tiagovilasboas/downshift/internal/routing/policy"
)

const policyApplyUsage = `usage: downshift policy apply --harness claude-code [--dry-run] [--yes]

Compile routing.profiles from the policy into harness-native agent files.
Currently supported harnesses: claude-code (writes <project>/.claude/agents/<name>.md,
where <project> is the directory that holds the .downshift/policy.json found from here).
  --harness   target harness (required: claude-code)
  --dry-run   show what would be written, write nothing
  --yes       do not ask before writing

A new agent file takes its description from the profile's "description"; a profile
without one cannot be created. A symlinked target is never written, and either every
file is written or none is.
`

type policyApplyOptions struct {
	Harness, Dir, Home string
	DryRun, Yes        bool
	In                 io.Reader
	Out, Err           io.Writer
	Now                func() time.Time
}

func (o policyApplyOptions) outf(format string, a ...any) {
	if o.Out != nil {
		fmt.Fprintf(o.Out, format, a...)
	}
}

func (o policyApplyOptions) errf(format string, a ...any) {
	if o.Err != nil {
		fmt.Fprintf(o.Err, format, a...)
	}
}

func runPolicyApply(args []string, in io.Reader, stdout, stderr io.Writer) int {
	opts := policyApplyOptions{In: in, Out: stdout, Err: stderr, Now: time.Now}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--harness" && i+1 < len(args):
			i++
			opts.Harness = args[i]
		case strings.HasPrefix(a, "--harness="):
			opts.Harness = strings.TrimPrefix(a, "--harness=")
		case a == "--dry-run":
			opts.DryRun = true
		case a == "--yes" || a == "-y":
			opts.Yes = true
		case a == "-h" || a == "--help":
			fmt.Fprint(stdout, policyApplyUsage)
			return 0
		default:
			fmt.Fprintf(stderr, "policy apply: unknown or missing argument %q\n%s", a, policyApplyUsage)
			return 2
		}
	}
	if opts.Harness == "" {
		fmt.Fprintf(stderr, "policy apply: --harness is required (supported: claude-code)\n%s", policyApplyUsage)
		return 2
	}
	if opts.Harness != "claude-code" {
		fmt.Fprintf(stderr, "policy apply: unsupported harness %q (supported: claude-code)\n", opts.Harness)
		return 2
	}
	var err error
	if opts.Dir, err = os.Getwd(); err != nil {
		fmt.Fprintf(stderr, "policy apply: cannot determine working directory: %v\n", err)
		return 1
	}
	if opts.Home, err = os.UserHomeDir(); err != nil {
		fmt.Fprintf(stderr, "policy apply: cannot determine home directory: %v\n", err)
		return 1
	}
	return executePolicyApply(opts)
}

// applyScope is what one run works on: the project root every agent path hangs
// from, and the profiles the merged policy declares for the harness.
type applyScope struct {
	root       string
	policyPath string
	profiles   []policy.Profile
}

// resolveApplyScope finds the nearest project policy and loads it together with
// the user's. The root is the directory that holds .downshift/policy.json, not
// the directory the command was started from. A policy that cannot be loaded is
// an error: nothing is read from a file that did not validate.
func resolveApplyScope(opts policyApplyOptions) (applyScope, error) {
	path, found, err := policyfile.Find(opts.Dir)
	if err != nil {
		return applyScope{}, err
	}
	if !found {
		return applyScope{}, fmt.Errorf("no %s found at or above %s; run `downshift policy init` in the project first",
			policyfile.RelPath, opts.Dir)
	}
	user := ""
	if opts.Home != "" {
		user = filepath.Join(opts.Home, ".downshift", policy.FileName)
	} else if u, err := policy.UserPath(); err == nil {
		user = u
	}
	loaded, err := policy.Load(policy.Locations{Project: path, User: user})
	if err != nil {
		return applyScope{}, err
	}
	scope := applyScope{root: projectRootOf(path), policyPath: path}
	for _, pr := range loaded.Profiles {
		if string(pr.Harness) == opts.Harness {
			scope.profiles = append(scope.profiles, pr)
		}
	}
	return scope, nil
}

// projectRootOf maps <root>/.downshift/policy.json back to <root>.
func projectRootOf(policyPath string) string {
	return filepath.Dir(filepath.Dir(policyPath))
}

func executePolicyApply(opts policyApplyOptions) int {
	scope, err := resolveApplyScope(opts)
	if err != nil {
		opts.errf("policy apply: %v\n", err)
		return 1
	}
	if len(scope.profiles) == 0 {
		opts.outf("policy apply: no routing.profiles entry for %s in %s; nothing to apply (profiles belong under the \"routing\" key)\n",
			opts.Harness, scope.policyPath)
		return 0
	}

	cat := catalog.Load()
	var plans []agentPlan
	var problems []error
	for _, pr := range scope.profiles {
		plan, err := planAgent(opts, scope.root, cat, pr)
		switch {
		case err != nil:
			problems = append(problems, err)
		case plan.unchanged():
			opts.outf("%s: already up to date (%s)\n", plan.name, plan.shown)
		default:
			plans = append(plans, plan)
		}
	}
	if len(problems) > 0 {
		for _, p := range problems {
			opts.errf("policy apply: %v\n", p)
		}
		opts.errf("policy apply: nothing was written\n")
		return 1
	}
	if len(plans) == 0 {
		return 0
	}

	verb := "will update"
	if opts.DryRun {
		verb = "would update"
	}
	for _, plan := range plans {
		opts.outf("%s: %s %s (model: %s)\n", plan.name, verb, plan.shown, plan.modelID)
		opts.outf("%s\n", unifiedDiff("a/"+plan.shown, "b/"+plan.shown, string(plan.before), plan.after))
	}
	if opts.DryRun {
		opts.outf("Dry run: nothing was written.\n")
		return 0
	}

	if !setupConfirm(setupOptions{In: opts.In, Out: opts.Out, Yes: opts.Yes}, "Apply changes? [y/N] ") {
		opts.outf("Not applied. Re-run with --yes to skip this question.\n")
		return 1
	}

	now := opts.Now
	if now == nil {
		now = time.Now
	}
	if err := commitPlans(plans, now().UTC().Format("20060102T150405.000000")); err != nil {
		opts.errf("policy apply: %v\n", err)
		return 1
	}
	for _, plan := range plans {
		opts.outf("%s: applied to %s (model: %s)\n", plan.name, plan.shown, plan.modelID)
	}
	return 0
}

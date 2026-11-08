// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/tiagovilasboas/downshift-ai/internal/policyfile"
	"github.com/tiagovilasboas/downshift-ai/internal/routing/policy"
)

// policyEconomyRemoved is printed by 'downshift policy economy': the command
// wrote a "mode" key that no routing step ever read, so it was removed.
const policyEconomyRemoved = `downshift policy economy was removed on 2026-11-02: no routing step read the "mode" key it wrote, so it never changed a model. A "mode" key already in policy.json is ignored; you can delete it.`

// runPolicy dispatches 'downshift policy'. check validates the routing policy
// the hooks read, without routing anything; init creates the project policy
// file after a yes.
func runPolicy(args []string, stdout, stderr io.Writer) int {
	if len(args) >= 1 && args[0] == "init" {
		return runPolicyInit(args[1:], os.Stdin, stdout, stderr)
	}
	if len(args) >= 1 && args[0] == "economy" {
		fmt.Fprintln(stderr, policyEconomyRemoved)
		return 1
	}
	if len(args) >= 1 && args[0] == "apply" {
		return runPolicyApply(args[1:], os.Stdin, stdout, stderr)
	}
	if len(args) != 1 || args[0] != "check" {
		fmt.Fprintln(stderr, "usage: downshift policy check | downshift policy init [--dry-run] [--yes] | downshift policy apply --harness claude-code [--dry-run] [--yes]")
		return 2
	}
	return runPolicyCheck(stdout, stderr)
}

func runPolicyCheck(stdout, stderr io.Writer) int {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(stderr, "policy check:", err)
		return 1
	}
	project, _, err := policyfile.Find(cwd)
	if err != nil {
		fmt.Fprintln(stderr, "policy check:", err)
		return 1
	}
	user, err := policy.UserPath()
	if err != nil {
		fmt.Fprintln(stderr, "policy check:", err)
		return 1
	}
	// The same files also hold the tier floor, and policyfile refuses keys it
	// does not know, so the floor reader must accept each file too. A missing
	// file is fine; any other failure to read one is reported.
	for _, path := range []string{project, user} {
		if path == "" {
			continue
		}
		if _, err := policyfile.Load(path); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			fmt.Fprintln(stderr, "policy check:", err)
			return 1
		}
	}
	p, err := policy.Load(policy.Locations{Project: project, User: user})
	if err != nil {
		fmt.Fprintln(stderr, "policy check:", err)
		return 1
	}
	fmt.Fprintf(stdout, "project file: %s\nuser file:    %s\n", orNone(project), orNone(user))
	fmt.Fprintf(stdout, "policy ok: %d profiles, %d rules; critical threshold %g, both critical %s, quota TTL %s\n",
		len(p.Profiles), len(p.Rules), p.CriticalThreshold, p.BothCritical, p.QuotaTTL)
	return 0
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func orNone(path string) string {
	if path == "" || !exists(path) {
		return "none"
	}
	return path
}

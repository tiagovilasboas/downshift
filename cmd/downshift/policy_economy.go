// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"time"

	"github.com/tiagovilasboas/downshift/internal/policyfile"
	"github.com/tiagovilasboas/downshift/internal/routing/policy"
)

const policyEconomyUsage = `usage: downshift policy economy on|off [--user] [--dry-run] [--yes]

Turn economy mode on or off. Economy lowers each default profile one tier, on
every harness: a task that touches a risk category never goes below mid, and a
declared profile, hint or pinned model still wins. The file holds the top-level
"mode" key: "economy" or "standard".
  --user      write the user's policy.json in the state directory instead
  --dry-run   show the change, write nothing
  --yes       do not ask before writing
`

// policyEconomyTarget labels the plan in the output, as policy init does.
var policyEconomyTarget = setupTarget{Name: "policy"}

// runPolicyEconomy is 'downshift policy economy on|off'. It shares the plan,
// confirm and write steps with 'downshift setup' and 'downshift policy init'.
func runPolicyEconomy(args []string, in io.Reader, stdout, stderr io.Writer) int {
	o := setupOptions{In: in, Out: stdout, Err: stderr, Now: time.Now}
	var state string
	user := false
	for _, a := range args {
		switch a {
		case "on", "off":
			if state != "" {
				fmt.Fprintf(stderr, "policy economy: give one of on or off\n%s", policyEconomyUsage)
				return 2
			}
			state = a
		case "--user":
			user = true
		case "--dry-run":
			o.DryRun = true
		case "--yes", "-y":
			o.Yes = true
		case "-h", "--help":
			fmt.Fprint(stdout, policyEconomyUsage)
			return 0
		default:
			fmt.Fprintf(stderr, "policy economy: unknown argument %q\n%s", a, policyEconomyUsage)
			return 2
		}
	}
	if state == "" {
		fmt.Fprint(stderr, policyEconomyUsage)
		return 2
	}
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(stderr, "policy economy: cannot find the home directory: %v\n", err)
		return 1
	}
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "policy economy: %v\n", err)
		return 1
	}
	o.Home = home
	o.ProjectDir = cwd
	path, err := policyEconomyPath(cwd, user)
	if err != nil {
		fmt.Fprintf(stderr, "policy economy: %v\n", err)
		return 1
	}
	mode := policy.ModeStandard
	if state == "on" {
		mode = policy.ModeEconomy
	}
	plan := planPolicyMode(path, mode)
	failed, _ := applyPlans(o, []setupPlan{plan})
	if failed {
		return 1
	}
	return 0
}

// policyEconomyPath is the file the toggle writes. Without --user it is the
// nearest project policy file, the one the hooks read, else the file policy
// init would create in the current directory. --user is the state directory's.
func policyEconomyPath(cwd string, user bool) (string, error) {
	if user {
		return policy.UserPath()
	}
	found, ok, err := policyfile.Find(cwd)
	if err != nil {
		return "", err
	}
	if ok {
		return found, nil
	}
	return policy.ProjectPath(cwd), nil
}

// planPolicyMode plans the file with its "mode" key set to mode. It never
// writes. An existing file must already be usable, so the toggle never changes
// a file the policy loaders would refuse; a missing file gets the version too.
func planPolicyMode(path string, mode policy.Mode) setupPlan {
	plan := setupPlan{target: policyEconomyTarget, path: path}
	before, err := os.ReadFile(path)
	switch {
	case err == nil:
		plan.before, plan.existed = before, true
	case errors.Is(err, fs.ErrNotExist):
	default:
		plan.err = err
		return plan
	}
	doc := map[string]json.RawMessage{}
	if plan.existed {
		if err := checkPolicyFile(path); err != nil {
			plan.err = err
			return plan
		}
		if err := json.Unmarshal(before, &doc); err != nil {
			plan.err = err
			return plan
		}
	} else {
		doc["version"] = json.RawMessage("1")
	}
	value, err := json.Marshal(string(mode))
	if err != nil {
		plan.err = err
		return plan
	}
	doc["mode"] = value
	plan.after, plan.err = encodePolicyDoc(doc)
	return plan
}

// checkPolicyFile runs the two loaders over one file: the tier floor decoder,
// which refuses unknown keys, and the routing loader.
func checkPolicyFile(path string) error {
	if _, err := policyfile.Load(path); err != nil {
		return err
	}
	_, err := policy.Load(policy.Locations{Project: path})
	return err
}

// encodePolicyDoc writes the document with two-space indent and a final
// newline. HTML characters in strings stay as they were.
func encodePolicyDoc(doc map[string]json.RawMessage) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return nil, fmt.Errorf("policy economy: %w", err)
	}
	return buf.Bytes(), nil
}

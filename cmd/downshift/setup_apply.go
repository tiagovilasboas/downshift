// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
)

// applyPlans shows every plan, asks once when a change would be written, then
// writes the changed plans. proceed is false when the caller must stop here: a
// dry run, nothing to change, or the question was declined. failed reports a
// plan that could not be planned or written; a declined question counts as
// failed, so the exit code is 1.
func applyPlans(o setupOptions, plans []setupPlan) (failed, proceed bool) {
	pending := 0
	unwritable := map[string]bool{} // files that failed to write
	for _, p := range plans {
		shown := setupDisplay(o.Home, p.path)
		switch {
		case p.err != nil:
			failed = true
			fmt.Fprintf(o.Err, "%s: %s: %v\n  nothing was written for %s\n\n", p.target.Name, shown, p.err, p.target.Name)
		case p.skip != "":
			fmt.Fprintf(o.Out, "%s: skipped: %s\n\n", p.target.Name, p.skip)
		case !p.changed():
			fmt.Fprintf(o.Out, "%s: %s already up to date\n", p.target.Name, shown)
			setupPrintNotes(o.Out, p.notes)
			fmt.Fprintln(o.Out)
		default:
			pending++
			verb := "will change"
			if o.DryRun {
				verb = "would change"
			}
			fmt.Fprintf(o.Out, "%s: %s %s\n", p.target.Name, shown, verb)
			fmt.Fprint(o.Out, unifiedDiff(diffName(shown, p.existed, "a"), diffName(shown, true, "b"), string(p.before), string(p.after)))
			setupPrintNotes(o.Out, p.notes)
			fmt.Fprintln(o.Out)
		}
	}

	if pending == 0 || o.DryRun {
		if o.DryRun && pending > 0 {
			fmt.Fprintln(o.Out, "Dry run: nothing was written.")
		}
		return failed, false
	}
	if !setupConfirm(o, "Write these changes? A backup of each existing file is kept. [y/N] ") {
		fmt.Fprintln(o.Out, "Not applied. Re-run with --yes to skip this question.")
		return true, false
	}
	stamp := o.Now().UTC().Format("20060102T150405.000000")
	for _, p := range plans {
		if !p.changed() {
			continue
		}
		if dep := failedCompanion(p, unwritable); dep != "" {
			failed = true
			unwritable[p.path] = true
			fmt.Fprintf(o.Err, "%s: %s not written: %s could not be written, and the config would point at it\n",
				p.target.Name, setupDisplay(o.Home, p.path), setupDisplay(o.Home, dep))
			continue
		}
		bak, err := setupWrite(p, stamp)
		if err != nil {
			failed = true
			unwritable[p.path] = true
			fmt.Fprintf(o.Err, "%s: write failed: %v\n", p.target.Name, err)
			continue
		}
		if bak != "" {
			fmt.Fprintf(o.Out, "%s: written. Backup: %s\n", p.target.Name, setupDisplay(o.Home, bak))
		} else {
			fmt.Fprintf(o.Out, "%s: written (new file).\n", p.target.Name)
		}
	}
	return failed, true
}

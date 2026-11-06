// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"fmt"

	"github.com/tiagovilasboas/downshift/internal/setup/configfile"
)

// ApplyPlans shows every plan, asks once when a change would be written, then
// writes the changed plans. proceed is false when the caller must stop here: a
// dry run, nothing to change, or the question was declined. failed reports a
// plan that could not be planned or written; a declined question counts as
// failed, so the exit code is 1.
func ApplyPlans(o Options, plans []Plan) (failed, proceed bool) {
	plans = RefuseLinkedPlans(o.LinkBases(), plans)
	pending := 0
	unwritable := map[string]bool{} // files that failed to write
	said := map[string]bool{}       // skip reasons already printed (one linked folder, one line)
	for _, p := range plans {
		shown := Display(o.Home, p.Path)
		switch {
		case p.Err != nil:
			failed = true
			fmt.Fprintf(o.Err, "%s: %s: %v\n  nothing was written for %s\n\n", p.Target.Name, shown, p.Err, p.Target.Name)
		case p.Skip != "":
			unwritable[p.Path] = true // a config must not point at a file left unwritten
			if key := p.Target.Name + p.Skip; !said[key] {
				said[key] = true
				fmt.Fprintf(o.Out, "%s: skipped: %s\n\n", p.Target.Name, p.Skip)
			}
		case !p.Changed():
			fmt.Fprintf(o.Out, "%s: %s already up to date\n", p.Target.Name, shown)
			PrintNotes(o.Out, p.Notes)
			fmt.Fprintln(o.Out)
		default:
			pending++
			verb := "will change"
			if o.DryRun {
				verb = "would change"
			}
			fmt.Fprintf(o.Out, "%s: %s %s\n", p.Target.Name, shown, verb)
			fmt.Fprint(o.Out, configfile.UnifiedDiff(DiffName(shown, p.Existed, "a"), DiffName(shown, true, "b"), string(p.Before), string(p.After)))
			PrintNotes(o.Out, p.Notes)
			fmt.Fprintln(o.Out)
		}
	}

	if pending == 0 || o.DryRun {
		if o.DryRun && pending > 0 {
			fmt.Fprintln(o.Out, "Dry run: nothing was written.")
		}
		return failed, false
	}
	if !Confirm(o, "Write these changes? A backup of each existing file is kept. [y/N] ") {
		fmt.Fprintln(o.Out, "Not applied. Re-run with --yes to skip this question.")
		return true, false
	}
	stamp := o.Now().UTC().Format("20060102T150405.000000")
	for _, p := range plans {
		if !p.Changed() {
			continue
		}
		if dep := failedCompanion(p, unwritable); dep != "" {
			failed = true
			unwritable[p.Path] = true
			fmt.Fprintf(o.Err, "%s: %s not written: %s could not be written, and the config would point at it\n",
				p.Target.Name, Display(o.Home, p.Path), Display(o.Home, dep))
			continue
		}
		bak, err := Write(p, stamp)
		if err != nil {
			failed = true
			unwritable[p.Path] = true
			fmt.Fprintf(o.Err, "%s: write failed: %v\n", p.Target.Name, err)
			continue
		}
		if bak != "" {
			fmt.Fprintf(o.Out, "%s: %s written. Backup: %s\n", p.Target.Name, Display(o.Home, p.Path), Display(o.Home, bak))
		} else {
			fmt.Fprintf(o.Out, "%s: %s written (new file).\n", p.Target.Name, Display(o.Home, p.Path))
		}
	}
	return failed, true
}

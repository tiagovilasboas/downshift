// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strings"
)

// compactFailedGoTestJSON compacts a failed `go test -json` run: every event
// line of a test that passed or was skipped (with no danger line in its
// output) and every package-level event of a package that passed is cut. All
// other lines stay byte for byte, so each failure keeps its events. A stream
// with a line that is not an event stays raw.
func compactFailedGoTestJSON(text string) (string, string) {
	events, ok := parseGoJSON(text)
	if !ok {
		return "", "go_test_json_failed_unparsed"
	}
	result := map[string]string{}
	danger := map[string]bool{}
	for _, e := range events {
		key := e.Package + "\x00" + e.Test
		switch e.Action {
		case "pass", "skip", "fail":
			result[key] = e.Action
		case "output":
			if reGoJSONDanger.MatchString(e.Output) || goFoldDanger.MatchString(e.Output) && e.Test == "" {
				danger[key] = true
			}
		}
	}
	var kept []string
	cutTests, okPkgs := 0, 0
	for _, e := range events {
		key := e.Package + "\x00" + e.Test
		cut := false
		if e.Test == "" {
			cut = result[key] == "pass" && !danger[key]
			if cut && e.Action == "pass" {
				okPkgs++
			}
		} else {
			cut = (result[key] == "pass" || result[key] == "skip") && !danger[key]
			if cut && e.Action == "pass" {
				cutTests++
			}
		}
		if !cut {
			kept = append(kept, e.raw)
		}
	}
	if okPkgs == 0 && cutTests == 0 {
		return "", "go_test_json_failed_run_no_gain"
	}
	head := fmt.Sprintf("Go test -json: ok: %d packages, %d passing tests cut (failures kept verbatim)\n", okPkgs, cutTests)
	return head + strings.Join(kept, "\n") + "\n", ""
}

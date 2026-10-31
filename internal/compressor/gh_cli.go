// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"regexp"
	"strings"
)

// GitHub CLI (gh) output compressors.
//
// RTK principle: keep the facts an agent decides on (PR title, state, check
// pass/fail, failing-check names), drop presentation. In a non-TTY pipe — the
// only way downshift's compactor sees a subprocess — gh already emits
// line-based plain text, so there is rarely a literal box to remove; but if a
// caller feeds TTY output with box-drawing characters, stripGhDecoration
// removes them before parsing. A failing check is never summarized away: its
// name and state survive, and gh checks exits non-zero on failure, which the
// top-level CompressExit already preserves raw.

// gh pr view (non-TTY) prints a block of "key:\tvalue" header lines, then a
// "--" separator, then the body. These are the fields worth keeping.
var (
	reGhPrField = regexp.MustCompile(`^([a-z-]+):\t(.*)$`)
	// Box-drawing / underline decoration from TTY rendering.
	reGhBoxChars = regexp.MustCompile("[\u2500-\u257F\u2580-\u259F]")
)

// ghPrKeepFields lists the header keys the compactor preserves, in output
// order. Everything else in the header block (url, labels churn, projects,
// milestone, additions/deletions) is dropped as decoration.
var ghPrKeepFields = []string{"title", "state", "number", "author"}

// isGhPrView detects non-TTY `gh pr view` output: it carries the title and
// state header fields in "key:\tvalue" form.
func isGhPrView(input []byte) bool {
	s := stripGhDecoration(string(input))
	hasTitle, hasState := false, false
	for _, line := range strings.Split(s, "\n") {
		m := reGhPrField.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		switch m[1] {
		case "title":
			hasTitle = true
		case "state":
			hasState = true
		}
	}
	return hasTitle && hasState
}

// compressGhPrView keeps title, state, number and author plus a one-line
// checks rollup when present, and drops the body and the rest of the header.
func compressGhPrView(input []byte) ([]byte, bool, string) {
	s := stripGhDecoration(string(input))

	fields := map[string]string{}
	for _, line := range strings.Split(s, "\n") {
		if m := reGhPrField.FindStringSubmatch(line); m != nil {
			if _, seen := fields[m[1]]; !seen {
				fields[m[1]] = strings.TrimSpace(m[2])
			}
		}
	}
	if fields["title"] == "" || fields["state"] == "" {
		return input, false, "gh_pr_view_missing_core_fields"
	}

	var b strings.Builder
	for _, k := range ghPrKeepFields {
		if v := fields[k]; v != "" {
			fmt.Fprintf(&b, "%s: %s\n", k, v)
		}
	}
	out := b.String()
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	// Fail-open safety: the body and the dropped header fields may carry a
	// danger word (a failing-check mention, an "error" in the description).
	// gh pr view exits zero even when the PR's checks fail, so the exit code
	// alone does not protect those lines. When any danger line of the input is
	// absent from the kept summary, keep the whole output raw.
	if ghDroppedDangerLine(input, []byte(out)) {
		return input, false, "gh_pr_view_danger_line_preserved"
	}
	return []byte(out), true, fmt.Sprintf("compressed gh pr view (state %s)", fields["state"])
}

// --- gh pr checks -----------------------------------------------------------

// gh pr checks (non-TTY) prints one tab-separated row per check:
//
//	<name>\t<pass|fail|pending|skipping|cancel>\t<elapsed>\t<url>[\t<bucket>]
//
// A trailing summary line may read "N/M checks passing" or similar.
var (
	reGhCheckRow = regexp.MustCompile(`^([^\t]+)\t(pass|fail|pending|skipping|cancel|successful|failing)\t`)
	// Terminal states that mean an agent must look (RTK highlights these).
	ghCheckFailureStates = map[string]bool{"fail": true, "failing": true, "cancel": true}
)

// isGhPrChecks detects `gh pr checks` output: at least one tab-separated check
// row with a recognized state in the second column.
func isGhPrChecks(input []byte) bool {
	for _, line := range strings.Split(string(input), "\n") {
		if reGhCheckRow.MatchString(line) {
			return true
		}
	}
	return false
}

// compressGhPrChecks keeps "<state> <name>" per check, dropping the elapsed
// time and URL columns. Failing/cancelled checks are prefixed with "FAIL" so
// an agent spots them immediately; passing ones use "ok". Rows are ordered
// failures first.
func compressGhPrChecks(input []byte) ([]byte, bool, string) {
	type check struct {
		name  string
		state string
		fail  bool
	}
	var checks []check
	for _, line := range strings.Split(strings.TrimSuffix(string(input), "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		m := reGhCheckRow.FindStringSubmatch(line)
		if m == nil {
			// A non-row line that is not a check means the shape is not the
			// plain table we expect; preserve everything.
			return input, false, "gh_pr_checks_unrecognized_line_preserved"
		}
		checks = append(checks, check{
			name:  strings.TrimSpace(m[1]),
			state: m[2],
			fail:  ghCheckFailureStates[m[2]],
		})
	}
	if len(checks) == 0 {
		return input, false, "gh_pr_checks_no_rows"
	}

	var b strings.Builder
	failures := 0
	// Failures first.
	for _, c := range checks {
		if c.fail {
			fmt.Fprintf(&b, "FAIL %s (%s)\n", c.name, c.state)
			failures++
		}
	}
	for _, c := range checks {
		if !c.fail {
			fmt.Fprintf(&b, "ok   %s\n", c.name)
		}
	}
	out := b.String()
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	if ghDroppedDangerLine(input, []byte(out)) {
		return input, false, "gh_pr_checks_danger_line_preserved"
	}
	reason := fmt.Sprintf("compressed gh pr checks (%d checks, %d failing)", len(checks), failures)
	return []byte(out), true, reason
}

// --- gh run list ------------------------------------------------------------

// gh run list (non-TTY) prints one tab-separated row per run:
//
//	<status>\t<conclusion>\t<title>\t<workflow>\t<branch>\t<event>\t<id>\t<elapsed>\t<age>
//
// status is e.g. completed/in_progress/queued; conclusion is
// success/failure/cancelled/skipped (empty while running).
var reGhRunRow = regexp.MustCompile(`^(completed|in_progress|queued|requested|waiting|pending)\t([^\t]*)\t([^\t]*)\t([^\t]*)\t([^\t]*)\t([^\t]*)\t(\d+)\t`)

// isGhRunList detects `gh run list` output: at least one tab-separated run row.
func isGhRunList(input []byte) bool {
	for _, line := range strings.Split(string(input), "\n") {
		if reGhRunRow.MatchString(line) {
			return true
		}
	}
	return false
}

// compressGhRunList keeps "<conclusion> <workflow> (<branch>) #<id>" per run,
// dropping the elapsed and age columns and the redundant status column when a
// conclusion is present. Failures are listed first and marked.
func compressGhRunList(input []byte) ([]byte, bool, string) {
	type run struct {
		concl, title, workflow, branch, id string
		fail                               bool
	}
	var runs []run
	for _, line := range strings.Split(strings.TrimSuffix(string(input), "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		m := reGhRunRow.FindStringSubmatch(line)
		if m == nil {
			return input, false, "gh_run_list_unrecognized_line_preserved"
		}
		concl := m[2]
		if concl == "" {
			concl = m[1] // still running: fall back to the status column
		}
		runs = append(runs, run{
			concl:    concl,
			title:    strings.TrimSpace(m[3]),
			workflow: strings.TrimSpace(m[4]),
			branch:   strings.TrimSpace(m[5]),
			id:       m[7],
			fail:     concl == "failure" || concl == "cancelled" || concl == "timed_out" || concl == "startup_failure",
		})
	}
	if len(runs) == 0 {
		return input, false, "gh_run_list_no_rows"
	}

	emit := func(b *strings.Builder, r run, marker string) {
		label := r.workflow
		if label == "" {
			label = r.title
		}
		fmt.Fprintf(b, "%s%s %s (%s) #%s\n", marker, r.concl, label, r.branch, r.id)
	}

	var b strings.Builder
	failures := 0
	for _, r := range runs {
		if r.fail {
			emit(&b, r, "FAIL ")
			failures++
		}
	}
	for _, r := range runs {
		if !r.fail {
			emit(&b, r, "")
		}
	}
	out := b.String()
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	if ghDroppedDangerLine(input, []byte(out)) {
		return input, false, "gh_run_list_danger_line_preserved"
	}
	return []byte(out), true, fmt.Sprintf("compressed gh run list (%d runs, %d failing)", len(runs), failures)
}

// stripGhDecoration removes box-drawing characters that gh emits only when
// rendering to a TTY, so the key/value parser sees plain text. It leaves all
// other bytes untouched.
func stripGhDecoration(s string) string {
	if !reGhBoxChars.MatchString(s) {
		return s
	}
	return reGhBoxChars.ReplaceAllString(s, "")
}

// ghDroppedDangerLine reports whether any danger-word line of input is missing
// from out. The gh compressors keep the facts an agent decides on (title,
// state, check names) but drop presentation (body, URLs, elapsed times); when
// a dropped line carries a danger word the summary does not reflect, the
// compressor must fail open and keep the raw output so no warning or failure
// is ever summarised away.
func ghDroppedDangerLine(input, out []byte) bool {
	for _, line := range strings.Split(string(input), "\n") {
		if line == "" || !reDangerWord.MatchString(line) {
			continue
		}
		if !strings.Contains(string(out), line) {
			return true
		}
	}
	return false
}

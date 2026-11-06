// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// goFoldDanger marks lines a fold must never summarise away or hide behind a
// PASS header: failures, panics, races, timeouts and build-error headers.
var goFoldDanger = regexp.MustCompile(`(?i)fail|panic|race|timeout|^# `)

// goCoverHeaderMin is the largest ok-package count whose header omits the
// coverage min and max; larger runs state them up front.
const goCoverHeaderMin = 3

// reGoCoverStandalone is the `coverage: N% of statements` line that package
// directory mode prints before the ok line, which repeats the same figure.
var reGoCoverStandalone = regexp.MustCompile(`^coverage: (\S+) of statements$`)

type goPlainPkg struct {
	name    string
	cover   string
	noTests bool
}

// foldGoTestPlain folds the `ok` / `?` lines of a clean plain go test run into
// a summary. A bare PASS line is redundant and dropped. Any other line (init
// output, warnings, a standalone coverage line) is kept verbatim, in order,
// after the summary line. A line with a danger word, or no package line at
// all, returns the input unchanged.
func foldGoTestPlain(input []byte) ([]byte, bool, string) {
	var pkgs []goPlainPkg
	var stray, standalone []string
	var okCount, noTestCount int
	for _, line := range strings.Split(string(input), "\n") {
		if strings.TrimSpace(line) == "" || line == "PASS" {
			continue
		}
		if m := reGoTestPass.FindStringSubmatch(line); m != nil {
			pkgs = append(pkgs, goPlainPkg{
				name:  m[1],
				cover: strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(m[3]), "of statements")),
			})
			okCount++
		} else if m := reGoTestNoTest.FindStringSubmatch(line); m != nil {
			pkgs = append(pkgs, goPlainPkg{name: m[1], noTests: true})
			noTestCount++
		} else if m := reGoTestCoverNoFiles.FindStringSubmatch(line); m != nil {
			pkgs = append(pkgs, goPlainPkg{name: m[1], noTests: true})
			noTestCount++
		} else if m := reGoCoverStandalone.FindStringSubmatch(line); m != nil {
			standalone = append(standalone, m[1])
			stray = append(stray, line)
		} else if goFoldDanger.MatchString(line) {
			return input, false, "go_test_danger_line_preserved"
		} else {
			stray = append(stray, line)
		}
	}
	if len(pkgs) == 0 {
		return input, false, "go_test_no_package_lines"
	}
	stray = dropRepeatedCoverLines(stray, standalone, pkgs)
	names := make([]string, len(pkgs))
	for i, p := range pkgs {
		names[i] = p.name
	}
	prefix := commonPathPrefix(names)
	minCov, maxCov, haveCov := goCoverRange(pkgs)
	// Per-package figures are always kept (dropping them loses which package
	// is low); the min/max in the header is a quick read for larger runs.
	haveCov = haveCov && okCount > goCoverHeaderMin

	var b strings.Builder
	if noTestCount > 0 {
		fmt.Fprintf(&b, "Go test: PASS (%d packages ok, %d without tests", okCount, noTestCount)
	} else {
		fmt.Fprintf(&b, "Go test: PASS (%d packages ok", okCount)
	}
	if haveCov {
		fmt.Fprintf(&b, ", coverage min %s%% max %s%%", minCov, maxCov)
	}
	b.WriteString(")")
	if prefix != "" {
		fmt.Fprintf(&b, " under %s", prefix)
	}
	b.WriteString("\n")
	if len(stray) > 0 {
		b.WriteString("Other output (kept verbatim):\n")
		for _, l := range stray {
			b.WriteString(l + "\n")
		}
	}
	for _, p := range pkgs {
		rel := strings.TrimPrefix(p.name, prefix)
		if p.noTests {
			b.WriteString("? " + rel + " [no test files]\n")
			continue
		}
		b.WriteString("ok " + rel)
		if p.cover != "" {
			b.WriteString(" " + p.cover)
		}
		b.WriteString("\n")
	}
	out := b.String()
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	reason := fmt.Sprintf("compressed %d passing packages into summary", len(pkgs))
	if noTestCount > 0 {
		reason = fmt.Sprintf("compressed %d packages (%d ok, %d without tests) into summary", len(pkgs), okCount, noTestCount)
	}
	if len(stray) > 0 {
		reason += fmt.Sprintf(", %d other lines kept", len(stray))
	}
	return []byte(out), true, reason
}

func goCoverValue(c string) (float64, bool) {
	if !strings.HasSuffix(c, "%") {
		return 0, false
	}
	v, err := strconv.ParseFloat(strings.TrimSuffix(c, "%"), 64)
	return v, err == nil
}

// goCoverRange returns the lowest and highest percentage among the packages,
// formatted as printed by go test, and false when none has a parseable one.
func goCoverRange(pkgs []goPlainPkg) (lo, hi string, ok bool) {
	var loV, hiV float64
	for _, p := range pkgs {
		v, parsed := goCoverValue(p.cover)
		if !parsed {
			continue
		}
		if !ok || v < loV {
			loV, lo = v, p.cover
		}
		if !ok || v > hiV {
			hiV, hi = v, p.cover
		}
		ok = true
	}
	return strings.TrimSuffix(lo, "%"), strings.TrimSuffix(hi, "%"), ok
}

// dropRepeatedCoverLines removes a standalone coverage line whose figure an ok
// line already carries, so no fact is lost.
func dropRepeatedCoverLines(stray, figures []string, pkgs []goPlainPkg) []string {
	if len(figures) == 0 {
		return stray
	}
	have := map[string]bool{}
	for _, p := range pkgs {
		have[p.cover] = true
	}
	kept := stray[:0:0]
	for _, l := range stray {
		if m := reGoCoverStandalone.FindStringSubmatch(l); m != nil && have[m[1]] {
			continue
		}
		kept = append(kept, l)
	}
	return kept
}

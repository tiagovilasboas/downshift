// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"strconv"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/lazyre"
)

// A passing test run has a count line. When the raw output of that run is kept
// (CCR), the count line can lose what the stored copy still holds: package
// names, per-package coverage and timings. ShortenPass is that kept form. It
// reads a summary a compressor already wrote, never the raw output, so a line
// the compressor did not recognise (kept verbatim in the summary) survives
// untouched. Callers use it only for a zero exit and only after the raw output
// was stored: without a stored copy the longer summary is what is shown.

var (
	// reGoPassHeader is the header foldGoTestPlain writes.
	reGoPassHeader = lazyre.New(`^Go test: PASS \((\d+) packages ok(?:, (\d+) without tests)?(?:, coverage min (\S+)% max (\S+)%)?\)(?: under \S+)?$`)
	reGoPassOKRow  = lazyre.New(`^ok \S+(?: (\S+)%)?$`)
	reGoPassNoRow  = lazyre.New(`^\? \S+ \[no test files\]$`)
	// rePassTiming is the trailing duration of a "Name: PASS (... in 0.42s)" line.
	rePassTiming = lazyre.New(`^([A-Za-z][A-Za-z /]*: PASS \(.*?) in (?:\d+(?:\.\d+)?(?:ms|s)|\d\d:\d\d(?:\.\d+)?)\)$`)
)

// ShortenPass returns the kept form of a passing-run summary and true, or the
// summary unchanged and false when it is not one it knows, is not whole, or
// the result would not be shorter. Counts are copied, never computed from
// anything but the summary itself.
func ShortenPass(summary []byte) ([]byte, bool) {
	text := string(summary)
	short, ok := shortenGoPass(text)
	if !ok {
		short, ok = trimPassTiming(text)
	}
	if !ok {
		short, ok = trimFamilyTiming(text)
	}
	if !ok || len(short) >= len(text) {
		return summary, false
	}
	return []byte(short), true
}

// trimPassTiming drops the duration from the first line of a pass summary.
func trimPassTiming(text string) (string, bool) {
	first, rest, _ := strings.Cut(text, "\n")
	m := rePassTiming.FindStringSubmatch(first)
	if m == nil {
		return text, false
	}
	return m[1] + ")\n" + rest, true
}

// shortenGoPass drops the per-package rows of a go test summary. The header
// already counts the packages; the stored copy has names, timings and the
// per-package coverage. The kept form states the coverage range (the lowest
// package is the one to act on). Anything that is not a trailing run of rows
// matching the header counts keeps the summary whole.
func shortenGoPass(text string) (string, bool) {
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	m := reGoPassHeader.FindStringSubmatch(lines[0])
	if m == nil || !strings.HasSuffix(text, "\n") {
		return text, false
	}
	body := lines[1:]
	i := len(body)
	var okRows, noRows int
	var covers []string
	for ; i > 0; i-- {
		if r := reGoPassOKRow.FindStringSubmatch(body[i-1]); r != nil {
			okRows++
			if r[1] != "" {
				covers = append(covers, r[1])
			}
		} else if reGoPassNoRow.MatchString(body[i-1]) {
			noRows++
		} else {
			break
		}
	}
	wantNo := 0
	if m[2] != "" {
		wantNo, _ = strconv.Atoi(m[2])
	}
	if okRows+noRows == 0 || strconv.Itoa(okRows) != m[1] || noRows != wantNo {
		return text, false
	}
	var b strings.Builder
	b.WriteString("Go test: PASS (" + m[1] + " packages ok")
	if wantNo > 0 {
		b.WriteString(", " + m[2] + " without tests")
	}
	switch {
	case m[3] != "":
		b.WriteString(", coverage min " + m[3] + "% max " + m[4] + "%")
	case len(covers) > 0:
		lo, hi, ok := goCoverRange(coverPkgs(covers))
		if !ok {
			return text, false // a coverage figure it cannot read stays in the rows
		}
		b.WriteString(", coverage min " + lo + "% max " + hi + "%")
	}
	b.WriteString(")\n")
	for _, l := range body[:i] {
		b.WriteString(l + "\n")
	}
	return b.String(), true
}

func coverPkgs(covers []string) []goPlainPkg {
	pkgs := make([]goPlainPkg, len(covers))
	for i, c := range covers {
		pkgs[i].cover = c + "%"
	}
	return pkgs
}

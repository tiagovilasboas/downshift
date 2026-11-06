// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"errors"
	"regexp"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/console/setupwizard"
	"github.com/tiagovilasboas/downshift/internal/console/shared/diffview"
	"github.com/tiagovilasboas/downshift/internal/setup"
)

// previewSetup runs the dry-run planner for a selection and returns its diff.
// It writes no file.
func previewSetup(base setup.Options, d setupDirs, sel setupwizard.Selection) ([]diffview.File, error) {
	o := selectedSetup(base, d, sel)
	var out, errw bytes.Buffer
	o.Out, o.Err, o.In = &out, &errw, nil
	o.DryRun, o.Yes = true, false
	if setupRun(o) != 0 {
		msg := strings.TrimSpace(errw.String())
		if msg == "" {
			msg = strings.TrimSpace(out.String())
		}
		return nil, errors.New(msg)
	}
	return parseSetupPreview(out.String()), nil
}

// changeHeader matches "<harness>: <path> would change", the line that opens a diff.
var changeHeader = regexp.MustCompile(`^\S+: (.+) would change$`)

// parseSetupPreview turns the dry-run text into one File per changed file.
// Every other line (notes, MCP steps, binaries) goes to a trailing "notes" File.
func parseSetupPreview(text string) []diffview.File {
	var files []diffview.File
	var notes []string
	inDiff := false
	for _, l := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		if m := changeHeader.FindStringSubmatch(l); m != nil {
			files = append(files, diffview.File{Path: m[1]})
			inDiff = true
			continue
		}
		switch {
		case l == "":
			inDiff = false
		case inDiff && len(files) > 0:
			f := &files[len(files)-1]
			if len(f.Lines) == 0 && (strings.HasPrefix(l, "--- ") || strings.HasPrefix(l, "+++ ")) {
				continue // the file name is the title
			}
			f.Lines = append(f.Lines, l)
		case l != "Dry run: nothing was written.":
			notes = append(notes, " "+l)
		}
	}
	if len(notes) > 0 {
		files = append(files, diffview.File{Path: "notes", Lines: notes})
	}
	return files
}

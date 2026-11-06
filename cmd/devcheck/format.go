// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import "bytes"

// formatHint is the line the Makefile printed under the unformatted files.
const formatHint = "gofmt: format the files above (gofmt -w <file>)\n"

// checkFormat runs gofmt -l . and fails when it lists any file or errors.
func checkFormat() ([]byte, bool) {
	out, err := combined(nil, "gofmt", "-l", ".")
	return formatResult(out, err)
}

// formatResult turns gofmt's listing into the stage output: nothing on a clean
// tree, the listed files and the hint otherwise.
func formatResult(out []byte, err error) ([]byte, bool) {
	listed := bytes.TrimRight(out, "\r\n")
	if err == nil && len(listed) == 0 {
		return nil, true
	}
	msg := append(append([]byte{}, listed...), '\n')
	if err != nil {
		msg = append(msg, []byte("gofmt: "+err.Error()+"\n")...)
	}
	return append(msg, formatHint...), false
}

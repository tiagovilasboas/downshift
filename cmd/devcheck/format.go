// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// formatHint is the line the Makefile printed under the unformatted files.
const formatHint = "gofmt: format the files above (gofmt -w <file>)\n"

// checkFormat runs gofmt -l . and fails when it lists any file or errors.
func checkFormat() ([]byte, bool) {
	out, err := combined(nil, gofmtBin(), "-l", ".")
	return formatResult(out, err)
}

// gofmtBin is the gofmt of the toolchain the go command runs (go.mod's
// toolchain line), not the first gofmt on PATH. gofmt's output changes between
// Go releases, so an older system gofmt reports, and `gofmt -w` rewrites, files
// that CI's gofmt (setup-go from go.mod) accepts. Falls back to PATH when
// `go env GOROOT` cannot answer.
func gofmtBin() string {
	out, err := exec.Command("go", "env", "GOROOT").Output()
	root := strings.TrimSpace(string(out))
	if err != nil || root == "" {
		return "gofmt"
	}
	return toolchainGofmt(root, runtime.GOOS)
}

// toolchainGofmt is the gofmt binary inside goroot for goos.
func toolchainGofmt(goroot, goos string) string {
	name := "gofmt"
	if goos == "windows" {
		name += ".exe"
	}
	return filepath.Join(goroot, "bin", name)
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

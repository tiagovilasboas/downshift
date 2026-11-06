// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

// Where setup keeps a generated wrapper and how a hook command is recognised
// as one setup wrote. The script itself is rendered in wrapper.go.

const wrapperDirName = "bin"

// wrapperExt is the script extension of the host OS.
func wrapperExt(goos string) string {
	if goos == "windows" {
		return ".cmd"
	}
	return ".sh"
}

// WrapperFile is the file name of the wrapper with the given stem on goos.
func WrapperFile(stem, goos string) string { return stem + wrapperExt(goos) }

// IsWrapperCommand reports whether command runs a wrapper with this stem from
// any directory, for either OS. A wrapper takes no arguments, so the whole
// string is the path. Used to recognise a hook the user registered by hand.
func IsWrapperCommand(command, stem string) bool {
	base := path.Base(strings.ReplaceAll(strings.TrimSpace(command), `\`, "/"))
	ext := path.Ext(base)
	return (ext == ".sh" || ext == ".cmd") && strings.TrimSuffix(base, ext) == stem
}

// ownsCommand reports whether command is the hook h written by a previous run
// or by hand: a wrapper in any directory, or `downshift <args>` through any
// binary path.
func ownsCommand(command string, h Hook) bool {
	if h.Wrapper != "" {
		return IsWrapperCommand(command, h.Wrapper)
	}
	return IsDownshiftCommand(command, h.Args)
}

// WrapperPath is where setup keeps the wrapper for h.
func WrapperPath(stateDir string, h Hook) string {
	return filepath.Join(stateDir, wrapperDirName, WrapperFile(h.Wrapper, runtime.GOOS))
}

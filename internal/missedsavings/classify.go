// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package missedsavings scans a harness's own transcripts for shell commands the
// compactor would have changed but that ran bare. Transcripts hold task text,
// so everything is read line by line in memory and only fixed codes, command
// families, binary names and counts leave the package: never argv, paths or
// arguments.
package missedsavings

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
	"github.com/tiagovilasboas/downshift-ai/internal/lazyre"
)

// Kind is what the compact policy makes of one recorded command.
type Kind int

const (
	// KindWrapped: the command already ran through `downshift compact`.
	KindWrapped Kind = iota
	// KindBare: the policy would wrap it, but it ran as written.
	KindBare
	// KindSkipped: the policy leaves it alone; Class.Reason says why.
	KindSkipped
)

// Class is the privacy-safe verdict for one command: a kind plus fixed codes.
type Class struct {
	Kind   Kind
	Reason string // skip code (execwrap.Skip* / Reason*), KindSkipped only
	Family string // allowlisted family code, KindBare only
	Binary string // first word of the command, KindBare only
	Meta   string // execwrap.MetacharKind code, metachar skips only
}

// probeExe stands in for the running binary: Decide only needs an absolute
// path to build the rewrite, which this package never runs or stores.
// os.TempDir is absolute on every OS, unlike a fixed POSIX path.
var probeExe = filepath.Join(os.TempDir(), "downshift")

// wrappedRe finds a command already routed through the compactor, whatever
// the quoting of the binary path or the name it was called by.
var wrappedRe = lazyre.New(`(^|[\s/'"])downshift'?\s+(compact|compactor|exec)(\s|$)`)

// binaryRe bounds what a report may print as a binary name.
var binaryRe = lazyre.New(`^[A-Za-z0-9._+-]{1,32}$`)

// Classify asks the shipped execwrap policy what it does with command.
func Classify(command string) Class {
	if wrappedRe.MatchString(command) {
		return Class{Kind: KindWrapped}
	}
	d := execwrap.Decide(command, probeExe)
	switch {
	case d.Reason == execwrap.ReasonAlreadyWrapped:
		return Class{Kind: KindWrapped}
	case d.Rewrite:
		return Class{Kind: KindBare, Family: string(d.Family), Binary: binaryOf(d.Argv)}
	}
	c := Class{Kind: KindSkipped, Reason: execwrap.SkipReason(d, command)}
	if c.Reason == execwrap.SkipMetachar {
		c.Meta = execwrap.MetacharKind(command)
	}
	return c
}

// binaryOf returns the basename of the first word of argv, or "other" when it
// is not a plain program name.
func binaryOf(argv string) string {
	first, _, _ := strings.Cut(argv, " ")
	base := path.Base(first)
	if !binaryRe.MatchString(base) {
		return "other"
	}
	return base
}

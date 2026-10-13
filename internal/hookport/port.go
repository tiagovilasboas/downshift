// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package hookport is the optional honor and usage seam between a harness
// hook and the CLI. It does not parse harness payloads and it does not record
// telemetry. A nil func means that hook does not send the signal: absence is
// not inference.
package hookport

import "github.com/tiagovilasboas/downshift/internal/core"

// HonorFunc reports the model a child actually ran, when the hook payload
// carries that field. A nil HonorFunc means this harness hook does not report
// the executed child model.
type HonorFunc func(raw []byte, binaryVersion string, r core.Resolver) (stdout any, note string)

// UsageFunc reports child token usage, when the hook payload carries it.
// A nil UsageFunc means this harness hook does not report child token usage.
type UsageFunc func(raw []byte, binaryVersion string, r core.Resolver) string

// Port is one harness's optional post-spawn observation. Honor and Usage stay
// nil until that harness's own hook sends the field.
type Port struct {
	ID    string
	Honor HonorFunc // nil: this harness hook does not report the executed child model
	Usage UsageFunc // nil: this harness hook does not report child token usage
	// UsageOut is Usage plus an optional stdout for the hook (the child-end
	// line). When set it runs instead of Usage.
	UsageOut UsageOutFunc
	// ParseChildEnd turns this harness's post-spawn or stop payload into the
	// shared ChildEnd contract (`downshift end <harness>`). Nil means no end
	// event is known for this harness: the generic runner records nothing.
	ParseChildEnd ParseChildEndFunc
	// EndLine wraps the child-end line in this harness's hook wire output.
	// Nil means the harness has no way to show it.
	EndLine func(line string) any
}

// UsageOutFunc is UsageFunc that may also return the hook's stdout. A nil
// stdout prints nothing.
type UsageOutFunc func(raw []byte, binaryVersion string, r core.Resolver) (stdout any, note string)

// Observation is the result of running one port func. Observed is false when
// the func was nil; the caller must not treat that as a measured zero.
type Observation struct {
	Observed bool
	Stdout   any
	Note     string
}

// ObserveHonor runs p.Honor. A nil func returns Observed false with no stdout
// and no note. A func that ran is observed even when the note is empty. A nil
// stdout interface becomes struct{}{} so json.Marshal emits {}.
func ObserveHonor(p Port, raw []byte, binaryVersion string, r core.Resolver) Observation {
	if p.Honor == nil {
		return Observation{}
	}
	stdout, note := p.Honor(raw, binaryVersion, r)
	if stdout == nil {
		stdout = struct{}{}
	}
	return Observation{Observed: true, Stdout: stdout, Note: note}
}

// ObserveUsage runs p.UsageOut when set, else p.Usage. A nil func returns Observed false, an empty note,
// and a nil stdout. A func that ran is observed and keeps the returned note,
// which may be empty. This function does not invent stdout.
func ObserveUsage(p Port, raw []byte, binaryVersion string, r core.Resolver) Observation {
	if p.UsageOut != nil {
		stdout, note := p.UsageOut(raw, binaryVersion, r)
		return Observation{Observed: true, Stdout: stdout, Note: note}
	}
	if p.Usage == nil {
		return Observation{}
	}
	return Observation{Observed: true, Note: p.Usage(raw, binaryVersion, r)}
}

// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package postrun compacts a tool's result after the tool ran, for a harness
// whose hook can replace that result before the model reads it
// (core.HarnessCapabilities.ReplacesToolOutput). It is harness-free: an
// adapter reads its payload into a hookport.ToolOutput and writes the answer
// back; this package only decides what the text becomes.
//
// Shell output goes through the same steps as `downshift compact`
// (execwrap.CompactCaptured): the family's compressor when the command is one
// plain allowlisted program, otherwise only the balanced cut, which needs the
// raw output stored in CCR and a gain of at least a quarter. Search results
// (matches and file lists) are cut only when larger than the balanced
// threshold, with the raw text stored first. File reads are never touched:
// an edit needs the file's exact text.
package postrun

import (
	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
	"github.com/tiagovilasboas/downshift-ai/internal/hookport"
)

// Env turns post-run compaction off when "0". It is on by default wherever
// the harness declares ReplacesToolOutput.
const Env = "DOWNSHIFT_POST_COMPACT"

// Entry is the exec log's entry code for a run compacted after it ran.
const Entry = "post_tool_use"

// FamilySearchTool is the exec log family of a search tool's result (a
// harness's own grep or glob tool, not a shell command).
const FamilySearchTool execwrap.Family = "search-tool"

// Options are the ports and switches the runner reads at the edge.
type Options struct {
	// Keep stores raw output for `downshift retrieve` (CCR). Nil means
	// nothing is stored, so nothing is cut with loss.
	Keep func(raw []byte) (id string, ok bool)
	// Balanced allows the balanced cut (DOWNSHIFT_BALANCED).
	Balanced bool
	// Session is the hashed session of the call, empty when unknown. With it
	// the marker's hint reads `downshift retrieve --session=<ref> <id>`.
	Session string
	// PreWrapped reports whether the harness's pre-run hook already wrapped
	// this shell command in `downshift compact`, so its output was compacted
	// there. Nil means no pre-run wrap is known.
	PreWrapped func(command string) bool
}

// Result is what Compact did with one tool result. It carries byte counts and
// codes for the exec log, never the text it logged nothing of.
type Result struct {
	// Considered is true when the output went through the compactor; the
	// runner logs one exec row for it. False: not this package's to handle
	// (wrapped before the run, an opaque result, nothing printed).
	Considered bool
	// Replaced is true when Output differs from the input and is shorter.
	Replaced bool
	Output   hookport.ToolOutput
	Family   execwrap.Family
	// RawBytes and EmittedBytes count the text fields before and after.
	RawBytes, EmittedBytes int64
	Compacted, Folded      bool
	Passthrough, Detail    string
	KeptID                 string
	ExitCode               int
}

// Compact returns the compaction of in, a finished tool call on a harness
// with caps. Every failure leaves the result unreplaced: a panic, a category
// the harness cannot replace or this package does not handle.
func Compact(caps core.HarnessCapabilities, in hookport.ToolOutput, o Options) (res Result) {
	defer func() {
		if recover() != nil {
			res = Result{}
		}
	}()
	if in.Opaque || !caps.ReplacesOutputOf(in.Category) {
		return Result{}
	}
	switch in.Category {
	case core.CategoryShell:
		return compactShell(in, o)
	case core.CategorySearch:
		return compactSearch(in, o)
	}
	return Result{} // core.CategoryRead stays whole: an edit needs the exact text
}

// compactShell compacts stdout as the wrapper would, and stderr only by the
// balanced cut when it alone is over the threshold.
func compactShell(in hookport.ToolOutput, o Options) Result {
	if !in.HasStdout || in.Stdout == "" && in.Stderr == "" {
		return Result{}
	}
	if execwrap.IsWrapped(in.Command) || execwrap.HasMarker(in.Stdout) || (o.PreWrapped != nil && o.PreWrapped(in.Command)) {
		return Result{}
	}
	exit := in.ExitCode
	if exit < 0 {
		exit = 0 // a post-run event for a successful call carries no code
	}
	family, argv := execwrap.SimpleFamily(in.Command)
	res := Result{Considered: true, Family: family, ExitCode: exit, Output: in}
	res.RawBytes = int64(len(in.Stdout) + len(in.Stderr))
	out := execwrap.CompactCaptured(argv, family, []byte(in.Stdout), exit, o.Keep, o.Balanced, o.Session)
	res.Compacted, res.Folded, res.Passthrough, res.Detail, res.KeptID = out.Compacted, out.Folded, out.Passthrough, out.Detail, out.KeptID
	if out.Compacted {
		res.Output.Stdout = string(out.Output)
	}
	if in.HasStderr && len(in.Stderr) > execwrap.BalanceThreshold {
		errOut := execwrap.CompactCaptured(nil, execwrap.FamilyNone, []byte(in.Stderr), exit, o.Keep, o.Balanced, o.Session)
		if errOut.Compacted {
			res.Output.Stderr = string(errOut.Output)
			res.Compacted, res.Folded, res.Passthrough, res.Detail = true, res.Folded || errOut.Folded, "", ""
			if res.KeptID == "" {
				res.KeptID = errOut.KeptID
			}
		}
	}
	res.EmittedBytes = int64(len(res.Output.Stdout) + len(res.Output.Stderr))
	return settle(res, in)
}

// settle undoes a result that would not shorten the output: the row then
// says the output stayed raw because the summary was not smaller.
func settle(res Result, in hookport.ToolOutput) Result {
	res.Replaced = res.Compacted && res.EmittedBytes < res.RawBytes
	if res.Replaced {
		return res
	}
	if res.Compacted {
		res.Compacted, res.Folded, res.KeptID, res.Passthrough, res.Detail = false, false, "", execwrap.PassthroughSmallOutput, ""
	}
	res.Output, res.EmittedBytes = in, res.RawBytes
	return res
}

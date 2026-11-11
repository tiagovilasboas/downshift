// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
	"github.com/tiagovilasboas/downshift-ai/internal/hookport"
	"github.com/tiagovilasboas/downshift-ai/internal/postrun"
	"github.com/tiagovilasboas/downshift-ai/internal/shellwrap"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

// postRunAnswer is the shared post-run step of the honor runner: on a harness
// that declares ReplacesToolOutput and whose adapter exposes the finished
// tool's result, it compacts that result (internal/postrun), logs one exec row
// (entry post_tool_use) and returns the adapter's answer carrying the
// replacement. Anything else, a panic included, returns prev unchanged: the
// tool already ran, so the hook only ever falls back to its usual answer.
func postRunAnswer(port hookport.Port, raw []byte, prev any) (out any) {
	defer func() {
		if recover() != nil {
			out = prev
		}
	}()
	caps, ok := core.CapsFor(port.ID)
	if !ok || len(caps.ReplacesToolOutput) == 0 || port.ParseToolOutput == nil || port.ReplaceToolOutput == nil || !postRunOn() {
		return prev
	}
	in, ok := port.ParseToolOutput(raw)
	if !ok {
		return prev
	}
	keep, _ := ccrKeep() // a successful call: only full CCR stores it
	res := postrun.Compact(caps, in, postrun.Options{
		Keep:       keep,
		Balanced:   loadBalanced().on,
		Session:    telemetry.HashSessionID(in.Session),
		PreWrapped: func(command string) bool { return shellwrap.WouldRewrite(port.ID, command) },
	})
	if res.Considered {
		recordPostRun(port.ID, in.Session, res)
	}
	if !res.Replaced {
		return prev
	}
	return port.ReplaceToolOutput(prev, res.Output)
}

// postRunOn reports whether post-run compaction is on: off with
// DOWNSHIFT_POST_COMPACT=0, and off whenever the Compactor itself is off
// (DOWNSHIFT_EXEC=0 or "exec": false).
func postRunOn() bool {
	return os.Getenv(postrun.Env) != "0" && os.Getenv(execEnvKillSwitch) != "0" && execConfigEnabled()
}

// recordPostRun appends the exec row of one post-run compaction: family, byte
// counts, exit class and the CCR id, like a wrapped run; never the text.
func recordPostRun(harness, session string, res postrun.Result) {
	path, err := execwrap.EventsPath()
	if err != nil {
		return
	}
	class := execwrap.ExitClassZero
	if res.ExitCode != 0 {
		class = execwrap.ExitClassNonZero
	}
	ev := execwrap.EventFrom(execwrap.Result{
		ExitCode: res.ExitCode, ExitClass: class, Family: res.Family,
		RawBytes: res.RawBytes, EmittedBytes: res.EmittedBytes,
		Compacted: res.Compacted, Folded: res.Folded,
		Passthrough: res.Passthrough, PassthroughDetail: res.Detail, KeptID: res.KeptID,
	}, time.Now())
	if session != "" {
		ev.Session = telemetry.HashSessionID(session)
	}
	ev.Harness, ev.Entry, ev.Client = harness, postrun.Entry, envClient()
	ev.Inner, ev.Kind = res.Inner, res.Kind
	_ = execwrap.Append(path, ev)
}

// Payload caps of the honor runner. Every hook reads at most 1 MiB, so a
// malformed or hostile stdin cannot exhaust memory. The post-run hook of a
// harness that can replace a tool's result reads up to 16 MiB instead: the
// result it compacts is in the payload, and the outputs worth cutting are the
// large ones (a 1.2 MB `git log -p` passed through whole under the 1 MiB cap).
// The read keeps its deadline, and any failure still leaves the result as is.
const (
	hookPayloadCap    int64 = 1 << 20
	postRunPayloadCap int64 = 16 << 20
)

// honorPayloadLimit is the most the honor runner reads for harness: the
// post-run cap where the harness declares it can replace tool output and
// post-run compaction is on, else the hook cap.
func honorPayloadLimit(harness string) int64 {
	if caps, ok := core.CapsFor(harness); ok && len(caps.ReplacesToolOutput) > 0 && postRunOn() {
		return postRunPayloadCap
	}
	return hookPayloadCap
}

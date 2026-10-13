// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/tiagovilasboas/downshift/internal/childend"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/hookport"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// runEnd is the shared child-end runner behind `downshift end <harness>`. The
// harness's own parser turns its post-spawn or stop payload into a
// hookport.ChildEnd; childend.Record writes the resolved, child_ended and usage
// records and builds the child-end line, for every harness alike. The child
// already finished, so nothing here blocks: a read error, an oversized payload,
// a panic, a harness with no end event, or a payload that is not a child end
// exits 0 and records nothing. Stdout is the harness's wire form of the
// child-end line, only when the harness can show it and DOWNSHIFT_QUIET is off.
func runEnd(in io.Reader, port hookport.Port, catalog core.Resolver) (rc int) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintln(os.Stderr, "downshift: internal error in end, ignored")
			rc = 0
		}
	}()
	if port.ParseChildEnd == nil {
		fmt.Fprintf(os.Stderr, "downshift: no end event known for %s\n", port.ID)
		return 0
	}
	const maxHookPayloadBytes = 1 << 20
	data, err := readHookPayload(in, maxHookPayloadBytes+1)
	if err != nil || len(data) > maxHookPayloadBytes {
		return 0
	}
	ce, ok := port.ParseChildEnd(data)
	if !ok {
		return 0
	}
	if ce.Harness == "" {
		ce.Harness = port.ID
	}
	res := childend.Record(ce, buildVersion, catalog)
	caps, _ := core.CapsFor(port.ID)
	if res.Line != "" && port.EndLine != nil && telemetry.ChildLineEnabled(caps.CanShowChildLine) {
		if encoded, err := json.Marshal(port.EndLine(res.Line)); err == nil {
			fmt.Fprintf(os.Stdout, "%s\n", encoded)
		}
	}
	if res.Note != "" {
		fmt.Fprintf(os.Stderr, "downshift: %s\n", res.Note)
	}
	return 0
}

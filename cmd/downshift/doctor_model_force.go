// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// checkModelForce fails when the harness is told to run every subagent on one
// model: then neither the orchestrator's per-call choice nor a Downshift
// rewrite reaches the child. It reads the env block of the hook file, which the
// harness passes to every session and which a doctor run from a terminal never
// sees in its own environment, and then the process environment.
func checkModelForce(add func(id, status, detail, fix string), caps core.HarnessCapabilities, raw any, file string, getenv func(string) string) {
	name := caps.ModelForceEnv
	if name == "" {
		return
	}
	where := ""
	if doc, ok := raw.(map[string]any); ok {
		if block, ok := doc["env"].(map[string]any); ok {
			if v, ok := block[name]; ok && flagOn(fmt.Sprint(v)) {
				where = file
			}
		}
	}
	if where == "" && getenv != nil && flagOn(getenv(name)) {
		where = "the environment"
	}
	if where == "" {
		return
	}
	add("subagent model", doctorFail,
		fmt.Sprintf("%s is on in %s: every subagent runs on one fixed model, so the orchestrator's choice and Downshift's rewrite are ignored", name, where),
		"remove "+name+" from "+where)
}

// flagOn reads an on/off variable the way the harness does: anything but
// empty, "0" or "false" is on.
func flagOn(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	return v != "" && v != "0" && v != "false"
}

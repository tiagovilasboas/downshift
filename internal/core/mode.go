// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"encoding/json"
	"os"

	"github.com/tiagovilasboas/downshift/internal/paths"
)

// ModeFile is the state file that selects the routing mode at runtime. The
// dock writes it; every hook reads it, because a hook is a new process and
// cannot see an environment change made after Claude Code started.
const ModeFile = "mode.json"

// economyFrom reports whether economy mode is on. The mode file wins when it is
// present and valid; without it, DOWNSHIFT_MODE decides. Anything else is
// performance, the default.
func economyFrom(getenv func(string) string) bool {
	if mode, ok := readModeFile(); ok {
		return mode == ModeEconomy
	}
	return getenv(ModeEnv) == ModeEconomy
}

// ModeEconomy and ModePerformance are the two values of the mode file.
const (
	ModeEconomy     = "economy"
	ModePerformance = "performance"
)

// readModeFile returns the mode in the state dir's mode file. ok is false when
// the file is missing or does not hold a known mode.
func readModeFile() (string, bool) {
	path, err := paths.Join(ModeFile)
	if err != nil {
		return "", false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	var f struct {
		Mode string `json:"mode"`
	}
	if json.Unmarshal(data, &f) != nil || (f.Mode != ModeEconomy && f.Mode != ModePerformance) {
		return "", false
	}
	return f.Mode, true
}

// CurrentMode is the mode a new hook call would use.
func CurrentMode() string {
	if economyFrom(os.Getenv) {
		return ModeEconomy
	}
	return ModePerformance
}

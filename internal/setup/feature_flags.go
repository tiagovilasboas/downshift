// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import "fmt"

// FeatureFlag is a switch in the harness's own TOML config that must be on
// for the Downshift hooks (or part of them) to run. It is a harness fact, so
// it lives on the target row; `doctor` reads it (cmd/downshift, which holds
// the TOML parser), and setup never writes it, because the file belongs to
// the harness.
//
// The value under Table.Key is on when it is the boolean true or a table
// whose "enabled" is true (`[features.multi_agent_v2] enabled = true`).
type FeatureFlag struct {
	File  string // TOML file relative to HOME, "/"-separated
	Table string // table that holds the key ("features")
	Key   string // the current key name
	// Aliases are older names the harness still reads with the same meaning.
	Aliases []string
	// Default is the value when the key is absent, in the harness's current
	// release. An older release may differ; DefaultNote says how.
	Default     bool
	DefaultNote string
	// AllHooks is true when the harness runs no hook at all with the flag off.
	// False means only part of the routing needs it (Purpose says which).
	AllHooks bool
	Purpose  string // what stops working with the flag off
}

// Line is the exact text that turns the flag on.
func (f FeatureFlag) Line() string { return fmt.Sprintf("[%s] %s = true", f.Table, f.Key) }

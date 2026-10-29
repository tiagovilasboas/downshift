// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package policy

// Mode is the routing mode a policy file declares with the top-level "mode"
// key. Economy lowers every default profile row one tier (decide applies it);
// standard leaves the default table as it is. A file that leaves the key out
// inherits the other file's mode, and standard is the default.
type Mode string

const (
	ModeStandard Mode = "standard"
	ModeEconomy  Mode = "economy"
)

// Valid reports whether m is a declared mode.
func (m Mode) Valid() bool { return m == ModeStandard || m == ModeEconomy }

// EconomyMode reports whether the merged policy runs in economy mode. Nil-safe:
// no policy is standard mode.
func (p *Policy) EconomyMode() bool { return p != nil && p.Mode == ModeEconomy }

// parseModeKey validates the raw "mode" value of one file. An absent key is
// nil, so the merge can fall back to the other file.
func parseModeKey(source string, raw *string) (*Mode, error) {
	if raw == nil {
		return nil, nil
	}
	m := Mode(*raw)
	if !m.Valid() {
		return nil, refuse(source, "mode", "%q is not standard or economy", *raw)
	}
	return &m, nil
}

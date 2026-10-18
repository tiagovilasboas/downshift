// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"crypto/sha256"
	"encoding/binary"
	"os"
	"strconv"
	"strings"
)

// ControlShareEnv names the variable that sets the control-group share.
const ControlShareEnv = "DOWNSHIFT_CONTROL_SHARE"

// DefaultControlShare is the fraction of spawns left unrouted on purpose so
// `downshift report` has a measured baseline. 0 disables the control group.
const DefaultControlShare = 0.10

// ReasonControl marks a spawn the built-in control group left unrouted:
// classified and logged, never rewritten (outcome "baseline"). It is distinct
// from ReasonPaused and from DOWNSHIFT_NO_ROUTE, which carry no code.
const ReasonControl = "CONTROL"

// ControlShare reads DOWNSHIFT_CONTROL_SHARE. Unset or unparseable falls back
// to the default; values are clamped to [0, 1]. "0" disables the group.
func ControlShare() float64 {
	raw := strings.TrimSpace(os.Getenv(ControlShareEnv))
	if raw == "" {
		return DefaultControlShare
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || v != v { // NaN
		return DefaultControlShare
	}
	switch {
	case v < 0:
		return 0
	case v > 1:
		return 1
	}
	return v
}

// InControlGroup reports whether the spawn with this routing id belongs to the
// control group. It is a pure function of the id and the share: the same id
// always lands on the same side, on every harness, and no state is kept.
func InControlGroup(id string, share float64) bool {
	if share <= 0 || id == "" {
		return false
	}
	if share >= 1 {
		return true
	}
	sum := sha256.Sum256([]byte("harness-downshift-control-v1:" + id))
	// Top 53 bits give a uniform float64 in [0, 1).
	u := float64(binary.BigEndian.Uint64(sum[:8])>>11) / (1 << 53)
	return u < share
}

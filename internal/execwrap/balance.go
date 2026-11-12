// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"bytes"

	"github.com/tiagovilasboas/downshift-ai/internal/compressor"
)

// BalanceThreshold is the default size above which balanced compaction cuts
// what the agent would read (Options.BalanceThreshold overrides it). Smaller
// outputs keep the lossless result.
const BalanceThreshold = 16 << 10

// balanceConfig is the balanced cut a run applies: Options.BalanceThreshold
// and Options.BalanceLimits, read from the config file by the caller.
type balanceConfig struct {
	threshold int // bytes; 0 means BalanceThreshold
	limits    compressor.BalanceLimits
}

// limit is the size above which the balanced cut applies.
func (c balanceConfig) limit() int {
	if c.threshold <= 0 {
		return BalanceThreshold
	}
	return c.threshold
}

// BalancedEnv turns balanced compaction off when "0" and on when "1",
// over the "balanced" key of the config file.
const BalancedEnv = "DOWNSHIFT_BALANCED"

// balanceRefused families are never cut with loss: a file read is the file
// itself, and agent-desktop reads have their own delta.
var balanceRefused = map[Family]bool{
	FamilyFileRead:     true,
	FamilyAgentDesktop: true,
}

// balanceKindOf is the shape a family's output is cut as.
func balanceKindOf(family Family) compressor.BalanceKind {
	switch family {
	case FamilyGitDiff, FamilyGitShow:
		return compressor.BalanceDiff
	case FamilyGrep, FamilyGitGrep, FamilyRg:
		return compressor.BalanceMatches
	}
	return compressor.BalanceLog
}

// balance cuts shown, what the run would print, when it is larger than the
// threshold of cfg (BalanceThreshold by default) and keep stores the raw output, so every cut line can be
// retrieved. shown may end in a marker line, which is replaced. It reports
// false, and the caller prints shown, when balancing is off for the family,
// the output is small, the cut saves too little or the raw output could not
// be stored. A panic counts as false.
func balance(keep func(raw []byte) (string, bool), family Family, raw, shown []byte, cfg balanceConfig) (res []byte, ok bool) {
	defer func() {
		if recover() != nil {
			res, ok = nil, false
		}
	}()
	threshold := cfg.threshold
	if threshold <= 0 {
		threshold = BalanceThreshold
	}
	if keep == nil || balanceRefused[family] || len(shown) <= threshold {
		return nil, false
	}
	body := shown
	if i := bytes.LastIndex(body, []byte(markerPrefix)); i >= 0 && bytes.IndexByte(body[i:], '\n') == len(body)-i-1 {
		body = body[:i]
	}
	cut, applied := compressor.BalanceWith(balanceKindOf(family), body, cfg.limits)
	if !applied {
		return nil, false
	}
	id, stored := keep(append([]byte(nil), raw...))
	if !stored {
		return nil, false
	}
	return append(append([]byte{}, cut...), MarkerWithID(len(raw), len(cut), id)...), true
}

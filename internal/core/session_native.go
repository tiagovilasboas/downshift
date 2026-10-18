// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"encoding/json"
	"io"
	"os"
	"regexp"
	"time"
)

const nativeSessionTTL = 5 * time.Minute

var nativeModelID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/+\-]*$`)

// LoadNativeSession consumes a complete current Cursor export offline. Presence
// is authoritative even on failure: stale/malformed native availability must
// not silently become an old discovery result or an operator's handwritten list.
func LoadNativeSession(harness string, now time.Time) (SessionList, bool) {
	caps, _ := CapsFor(harness)
	return loadNativeSession(caps, now)
}

func loadNativeSession(caps HarnessCapabilities, now time.Time) (SessionList, bool) {
	if caps.NativeSessionEnv == "" {
		return SessionList{}, false
	}
	path := nativeSessionPathEnv(caps.NativeSessionEnv)
	if path == "" {
		return SessionList{}, false
	}
	hold := UnknownSession()
	hold.NativeAvailability, hold.QuotaRequired = true, true
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return hold, true
	}
	f, err := os.Open(path)
	if err != nil {
		return hold, true
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, (1<<20)+1))
	if err != nil || len(data) > 1<<20 {
		return hold, true
	}
	var payload struct {
		ObservedAt time.Time `json:"observed_at"`
		Complete   bool      `json:"available_models_complete"`
		Models     *[]string `json:"available_models"`
	}
	if json.Unmarshal(data, &payload) != nil || !payload.Complete || payload.Models == nil ||
		payload.ObservedAt.IsZero() || payload.ObservedAt.After(now) ||
		!now.Before(payload.ObservedAt.Add(nativeSessionTTL)) {
		return hold, true
	}
	seen := map[string]bool{}
	for _, id := range *payload.Models {
		if len(id) > 256 || !nativeModelID.MatchString(id) || seen[id] {
			return hold, true
		}
		seen[id] = true
	}
	if ParseNativeQuotaHook == nil {
		return hold, true
	}
	usage, err := ParseNativeQuotaHook(caps.NativeUsageSource, data, now, nativeSessionTTL)
	if err != nil {
		return hold, true
	}
	session := KnownSession(*payload.Models)
	session.NativeAvailability, session.QuotaRequired = true, true
	session.Usage, session.QuotaTime = &usage, now
	return session, true
}

// nativeSessionTarget uses catalog metadata through Resolver, never provider
// list position, name spelling, or pool membership as a capability estimate.
// Unknown metadata cannot become a routing target. Equal-tier price ties retain
// the provider's order without treating that order as a quality signal.
func nativeSessionTarget(d Decision, res Resolver, session SessionList) (string, Model, bool, bool) {
	if res == nil || (d.Verdict == VerdictOK && !session.Blocks(d.Baseline.CurrentModel.ID) && !session.Blocks(d.Baseline.RequestedID)) {
		return "", Model{}, false, false
	}
	upshift := d.Verdict == VerdictUpshift
	strongest := upshift && (d.Baseline.Guessed || ExplicitUpshiftEnabled())
	var current Model
	if d.Verdict == VerdictDownshift {
		currentID := d.Baseline.RequestedID
		if currentID == "" {
			currentID = d.Baseline.CurrentModel.ID
		}
		var known bool
		current, known = res.LookupByID(d.Harness, currentID)
		if !known {
			return "", Model{}, false, false
		}
	}
	var best Model
	found := false
	quotaHeld := false
	for _, id := range session.IDs {
		// A closed gate is not a write. Opening it only answers whether this
		// id would have been the target, so a metadata miss is not a credit hold.
		allowed := session.QuotaAllows(d.Harness, id)
		check := session
		if !allowed {
			check.QuotaRequired = false
		}
		if !CanWriteSessionID(d.Harness, id, check, res) ||
			(res.IsExplicitOnly(d.Harness, id) && !upshift) {
			continue
		}
		m, ok := res.LookupByID(d.Harness, id)
		if !ok || m.ID == "" || m.Tier < d.Tier || m.Tier > TierFrontier {
			continue
		}
		// Exhausting a cheaper pool must not turn a downshift into an upgrade.
		if d.Verdict == VerdictDownshift && (m.Tier > current.Tier ||
			(m.Tier == current.Tier && (m.InputM <= 0 || m.OutputM <= 0 || current.InputM <= 0 || current.OutputM <= 0 || m.OutputM > current.OutputM))) {
			continue
		}
		if !allowed {
			quotaHeld = true
			continue
		}
		m.ID, m.Harness, m.Native = id, d.Harness, ""
		better := !found || (!strongest && m.Tier < best.Tier) || (strongest && m.Tier > best.Tier)
		if found && m.Tier == best.Tier && m.InputM > 0 && m.OutputM > 0 && best.InputM > 0 && best.OutputM > 0 && m.OutputM < best.OutputM {
			better = true
		}
		if better {
			best, found = m, true
		}
	}
	if !found {
		return "", Model{}, false, quotaHeld
	}
	return best.ID, best, true, false
}

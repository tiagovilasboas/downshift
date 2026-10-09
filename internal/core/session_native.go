// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"encoding/json"
	"io"
	"os"
	"regexp"
	"time"

	"github.com/tiagovilasboas/downshift/internal/quota"
)

const nativeSessionTTL = 5 * time.Minute

var nativeModelID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/+\-]*$`)

// LoadNativeSession consumes a complete current Cursor export offline. Presence
// is authoritative even on failure: stale/malformed native availability must
// not silently become an old discovery result or an operator's handwritten list.
func LoadNativeSession(harness string, now time.Time) (SessionList, bool) {
	path := os.Getenv("DOWNSHIFT_CURSOR_NATIVE_FILE")
	if harness != "cursor" || path == "" {
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
	usage, err := quota.ParseNative("cursor-usage", data, now, nativeSessionTTL)
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
func nativeSessionTarget(d Decision, res Resolver, session SessionList) (string, Model, bool) {
	if res == nil || (d.Verdict == VerdictOK && !session.Blocks(d.CurrentModel.ID) && !session.Blocks(d.RequestedID)) {
		return "", Model{}, false
	}
	upshift := d.Verdict == VerdictUpshift
	var current Model
	if d.Verdict == VerdictDownshift {
		currentID := d.RequestedID
		if currentID == "" {
			currentID = d.CurrentModel.ID
		}
		var known bool
		current, known = res.LookupByID(d.Harness, currentID)
		if !known {
			return "", Model{}, false
		}
	}
	var best Model
	found := false
	for _, id := range session.IDs {
		if !CanWriteSessionID(d.Harness, id, session, res) ||
			(res.IsExplicitOnly(d.Harness, id) && !upshift) {
			continue
		}
		m, ok := res.LookupByID(d.Harness, id)
		if !ok || m.ID == "" || m.Tier < d.Tier || m.Tier > TierFrontier {
			continue
		}
		// Exhausting a cheaper pool must not turn a downshift into an upgrade.
		if d.Verdict == VerdictDownshift && (m.Tier > current.Tier ||
			(m.Tier == current.Tier && m.OutputM > current.OutputM)) {
			continue
		}
		m.ID, m.Harness, m.Native = id, d.Harness, ""
		better := !found || (!upshift && m.Tier < best.Tier) || (upshift && m.Tier > best.Tier)
		if found && m.Tier == best.Tier && m.OutputM < best.OutputM {
			better = true
		}
		if better {
			best, found = m, true
		}
	}
	return best.ID, best, found
}

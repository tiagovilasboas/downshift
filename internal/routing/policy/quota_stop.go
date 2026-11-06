// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"fmt"
	"strconv"
	"strings"
)

// The quota stop line: the share of a provider's quota window that, once used,
// makes delegate refuse to start more workers. Precedence, highest first: the
// user's override (DOWNSHIFT_QUOTA_STOP_PCT), the policy file
// ("orchestration": {"quota_stop_pct": N}), the default. A provider whose
// quota is unknown never stops a call: doubt stays allow.
const (
	// DefaultQuotaStopPct leaves a tenth of the window for the orchestrator itself.
	DefaultQuotaStopPct = 90.0
	// QuotaStopEnv is the user's override.
	QuotaStopEnv = "DOWNSHIFT_QUOTA_STOP_PCT"
)

// QuotaReading is what the stop line needs from one provider's quota window.
// Known is false when the headroom could not be read.
type QuotaReading struct {
	Provider    string
	UsedPct     float64
	RateLimited bool
	Known       bool
}

// QuotaStop says why a call must not start workers.
type QuotaStop struct {
	Provider    string
	UsedPct     float64
	Line        float64
	RateLimited bool
}

// Reason is the sentence returned to the caller.
func (s QuotaStop) Reason() string {
	if s.RateLimited {
		return fmt.Sprintf("%s quota is rate limited (%.0f%% used)", s.Provider, s.UsedPct)
	}
	return fmt.Sprintf("%s quota is %.0f%% used, at or past the stop line of %.0f%%", s.Provider, s.UsedPct, s.Line)
}

// CheckQuotaStop returns the first reading that stops the call: rate limited,
// or known and used at or past line. An unknown reading is allowed.
func CheckQuotaStop(line float64, readings []QuotaReading) (QuotaStop, bool) {
	for _, r := range readings {
		if r.RateLimited || (r.Known && r.UsedPct >= line) {
			return QuotaStop{Provider: r.Provider, UsedPct: r.UsedPct, Line: line, RateLimited: r.RateLimited}, true
		}
	}
	return QuotaStop{}, false
}

// ResolveQuotaStop applies the precedence. A zero value means "not set".
func ResolveQuotaStop(user, file float64) float64 {
	switch {
	case user > 0:
		return user
	case file > 0:
		return file
	}
	return DefaultQuotaStopPct
}

func validQuotaStop(n float64, err error) bool { return err == nil && n > 0 && n <= 100 }

// EnvQuotaStop reads the user's override. An invalid value is no override and
// returns a note.
func EnvQuotaStop(getenv func(string) string) (float64, []string) {
	raw := strings.TrimSpace(getenv(QuotaStopEnv))
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.ParseFloat(raw, 64)
	if !validQuotaStop(n, err) {
		return 0, []string{fmt.Sprintf("%s=%q ignored: want a number above 0 up to 100", QuotaStopEnv, raw)}
	}
	return n, nil
}

// FilesQuotaStop reads orchestration.quota_stop_pct from the project file, then
// the user file. An invalid value is skipped with a note and the next file is
// tried; an absent or unreadable file holds no value.
func FilesQuotaStop(loc Locations) (float64, []string) {
	var notes []string
	for _, path := range []string{loc.Project, loc.User} {
		n, note := fileQuotaStop(path)
		if note != "" {
			notes = append(notes, note)
		}
		if n > 0 {
			return n, notes
		}
	}
	return 0, notes
}

func fileQuotaStop(path string) (float64, string) {
	raw, note := orchestrationField(path, "quota_stop_pct")
	if raw == "" {
		return 0, note
	}
	n, err := strconv.ParseFloat(raw, 64)
	if !validQuotaStop(n, err) {
		return 0, fmt.Sprintf("%s: orchestration.quota_stop_pct %s ignored: want a number above 0 up to 100", path, raw)
	}
	return n, ""
}

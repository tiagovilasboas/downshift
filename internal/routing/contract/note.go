// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package contract

import "regexp"

// Verdict says which precedence step decided (design section 2 precedence and
// routing note; REA section 2 verdicts). Exactly one per decision.
type Verdict string

const (
	// VerdictKept: nothing applied, the requested model is kept (precedence 1 or 5).
	VerdictKept Verdict = "kept"
	// VerdictProfile: a named or agent-type profile set the tier (precedence 2).
	VerdictProfile Verdict = "profile"
	// VerdictRule: the first matching rule set the tier (precedence 3).
	VerdictRule Verdict = "rule"
	// VerdictHintDown: the parent's hint moved the tier down (precedence 4).
	VerdictHintDown Verdict = "hint_down"
	// VerdictGuardFloor: the guard raised the decided tier back to a floor.
	VerdictGuardFloor Verdict = "guard_floor"
	// VerdictHeldUnknown: a step chose a tier but it could not be resolved to a
	// model the harness honours, so the requested model is held (doubt keeps).
	VerdictHeldUnknown Verdict = "held_unknown"
	// VerdictRefusedConfig: the request or policy was a config error; the spawn
	// proceeds unchanged and the refusal is logged (REA: config errors refuse).
	VerdictRefusedConfig Verdict = "refused_config"
)

// Verdicts lists every Verdict in a stable order.
func Verdicts() []Verdict {
	return []Verdict{VerdictKept, VerdictProfile, VerdictRule, VerdictHintDown,
		VerdictGuardFloor, VerdictHeldUnknown, VerdictRefusedConfig}
}

// Valid reports whether v is one of Verdicts.
func (v Verdict) Valid() bool { return member(v, Verdicts()) }

// Reason refines a guard, hold or refusal verdict (design section 2 guard
// bullets; REA section 2 refusals). ReasonNone for the other verdicts.
type Reason string

const (
	ReasonNone            Reason = ""
	ReasonRiskFloor       Reason = "risk_floor"       // risk vocabulary: never small
	ReasonNeverWiden      Reason = "never_widen"      // target not honoured by the harness
	ReasonExplicitOnly    Reason = "explicit_only"    // catalog routing:"explicit_only"
	ReasonForeignID       Reason = "foreign_id"       // id from another harness
	ReasonProfileOverride Reason = "profile_override" // profile plus a pinned model or effort
	ReasonUnknownProfile  Reason = "unknown_profile"  // named profile not in the policy
	ReasonNoCandidate     Reason = "no_candidate"     // tier has no model in the candidate set
	ReasonFailOpen        Reason = "fail_open"        // error, unknown field or timeout
)

// Reasons lists every non-empty Reason in a stable order.
func Reasons() []Reason {
	return []Reason{ReasonRiskFloor, ReasonNeverWiden, ReasonExplicitOnly, ReasonForeignID,
		ReasonProfileOverride, ReasonUnknownProfile, ReasonNoCandidate, ReasonFailOpen}
}

// Valid reports whether r is one of Reasons.
func (r Reason) Valid() bool { return member(r, Reasons()) }

// HeadroomVerdict is the REA's chosen/ignored pair, read deterministically:
// chosen when the provider the decision landed on was under the strain
// threshold, ignored when a fresh reading put it past the threshold and the
// decision still landed there (for example a pinned model kept). It is advice
// for the note and never reroutes. Unknown when no fresh reading exists.
type HeadroomVerdict string

const (
	HeadroomUnknown HeadroomVerdict = ""
	HeadroomChosen  HeadroomVerdict = "chosen"
	HeadroomIgnored HeadroomVerdict = "ignored"
)

// HeadroomVerdicts lists every non-empty HeadroomVerdict.
func HeadroomVerdicts() []HeadroomVerdict { return []HeadroomVerdict{HeadroomChosen, HeadroomIgnored} }

// Valid reports whether h is one of HeadroomVerdicts.
func (h HeadroomVerdict) Valid() bool { return member(h, HeadroomVerdicts()) }

// Channel is how the decision reaches the model (design section 3).
type Channel string

const (
	ChannelUnknown    Channel = ""
	ChannelHook       Channel = "hook"
	ChannelLaunch     Channel = "launch"
	ChannelCompile    Channel = "compile"
	ChannelBlockRetry Channel = "block_retry"
)

// Channels lists every non-empty Channel.
func Channels() []Channel {
	return []Channel{ChannelHook, ChannelLaunch, ChannelCompile, ChannelBlockRetry}
}

// Valid reports whether c is one of Channels.
func (c Channel) Valid() bool { return member(c, Channels()) }

// identPattern bounds the names a policy may declare, so a rule id or profile
// name in a Note can never carry prose, a path or a prompt. routing/policy
// refuses any other name at load (design section 2: config errors at load).
var identPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{0,63}$`)

// RuleID is the id of a declared rule. Valid names match ^[a-z0-9][a-z0-9_.-]{0,63}$.
type RuleID string

// Valid reports whether id is a well-formed rule id.
func (id RuleID) Valid() bool { return identPattern.MatchString(string(id)) }

// ProfileName is the name of a declared profile; same shape as RuleID.
type ProfileName string

// Valid reports whether p is a well-formed profile name.
func (p ProfileName) Valid() bool { return identPattern.MatchString(string(p)) }

// Note is the routing note of one decision, for telemetry and status
// surfaces (design section 2 "Routing note"; REA section 2 routing note,
// section 8 lesson 5). Fields are enums, tiers, numbers, booleans and times
// only: no task text, no path, no prompt. A test fails if a free-text field
// is added. HeadroomUsedPct is meaningful only when HeadroomWindow is set.
type Note struct {
	Verdict         Verdict         `json:"verdict"`
	Reason          Reason          `json:"reason,omitempty"`
	RuleID          RuleID          `json:"rule_id,omitempty"`
	Profile         ProfileName     `json:"profile,omitempty"`
	TierFrom        Tier            `json:"tier_from,omitempty"`
	TierTo          Tier            `json:"tier_to,omitempty"`
	Harness         HarnessID       `json:"harness,omitempty"`
	Channel         Channel         `json:"channel,omitempty"`
	HeadroomWindow  Window          `json:"headroom_window,omitempty"`
	HeadroomUsedPct float64         `json:"headroom_used_pct,omitempty"`
	RateLimited     bool            `json:"rate_limited,omitempty"`
	Headroom        HeadroomVerdict `json:"headroom,omitempty"`
}

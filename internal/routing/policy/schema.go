// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package policy

// The JSON shapes of the routing section. They are decode-only: validate.go
// turns them into Policy values and refuses anything that does not fit.
// Pointers mark the settings whose absence must be told apart from a zero.

// section is the value of the "routing" key.
type section struct {
	CriticalThreshold *float64     `json:"critical_threshold"`
	BothCritical      *string      `json:"both_critical"`
	QuotaTTLSeconds   *int64       `json:"quota_ttl_seconds"`
	Profiles          []profileDoc `json:"profiles"`
	Rules             []ruleDoc    `json:"rules"`
}

type profileDoc struct {
	Name        string `json:"name"`
	Tier        string `json:"tier"`
	Harness     string `json:"harness"`
	Fallback    string `json:"fallback"`
	Description string `json:"description"`
}

type ruleDoc struct {
	ID      string    `json:"id"`
	Enabled *bool     `json:"enabled"`
	Kinds   []string  `json:"kinds"`
	Efforts []string  `json:"efforts"`
	Gates   []gateDoc `json:"gates"`
	Profile string    `json:"profile"`
	Tier    string    `json:"tier"`
}

type gateDoc struct {
	Harness string   `json:"harness"`
	Window  string   `json:"window"`
	Below   *float64 `json:"below"`
}

// maxBytes bounds one policy file read; a routing policy is a few kilobytes.
const maxBytes = 64 << 10

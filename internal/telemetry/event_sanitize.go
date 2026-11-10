// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/lazyre"
)

var correlationIDPattern = lazyre.New(`^[a-f0-9]{16,64}$`)
var modelIDPattern = lazyre.New(`^[A-Za-z0-9._:/-]{1,128}$`)
var rawModelPattern = lazyre.New(`^[A-Za-z0-9._:/-]{1,64}$`)
var effortPattern = lazyre.New(`^(none|minimal|low|medium|high|xhigh|max|ultra)$`)

// Session IDs are untrusted hook input. Validate a deliberately small opaque
// identifier vocabulary before hashing so arbitrary user-controlled payloads
// never enter even a derived local record.
var sessionIDPattern = lazyre.New(`^[A-Za-z0-9._:-]{1,128}$`)

// NewCorrelationID returns an opaque, prompt-free ID that joins one hook
// invocation to its event-log record.
func NewCorrelationID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

// CorrelationIDOrNew accepts only the opaque hexadecimal format. Invalid
// caller input is replaced rather than copied into local observability data.
func CorrelationIDOrNew(candidate string) string {
	if correlationIDPattern.MatchString(candidate) {
		return candidate
	}
	return NewCorrelationID()
}

// ModelOrUnknown bounds untrusted hook metadata before durable logging.
func ModelOrUnknown(candidate string) string {
	if modelIDPattern.MatchString(candidate) {
		return candidate
	}
	return "unknown"
}

// RawModelOrInvalid bounds an unrecognised requested model before logging.
// Anything that does not look like a short model identifier (for example
// text with spaces) becomes "invalid", so prompt text never reaches the log.
func RawModelOrInvalid(candidate string) string {
	if rawModelPattern.MatchString(candidate) {
		return candidate
	}
	return "invalid"
}

// clientPattern bounds the harness client label: a short, lower-case token.
// CLAUDE_CODE_ENTRYPOINT is untrusted, so anything with other characters or
// over the length cap is dropped rather than stored.
var clientPattern = lazyre.New(`^[a-z0-9._-]{1,32}$`)

// ClientOrDrop returns candidate when it is a short lower-case token
// (at most 32 chars, [a-z0-9._-]), and "" otherwise so the field is dropped.
// It never trusts the value for policy; it only labels the event.
func ClientOrDrop(candidate string) string {
	if clientPattern.MatchString(candidate) {
		return candidate
	}
	return ""
}

var parentSourcePattern = lazyre.New(`^[a-z0-9_-]{1,32}$`)

// ParentModelLabel bounds the parent model and its source before logging. An
// unusable model drops both, so a source never appears without its model.
func ParentModelLabel(model, source string) (string, string) {
	if !rawModelPattern.MatchString(model) {
		return "", ""
	}
	if !parentSourcePattern.MatchString(source) {
		source = ""
	}
	return model, source
}

// Reasons recorded in place of a reasoning effort level.
const (
	// EffortInherit is a spawn that sets no effort of its own, so the harness
	// applies its default (Claude Code: the session effort).
	EffortInherit = "inherit"
	// EffortUnobserved is a hook payload that carries no effort at all.
	EffortUnobserved = "unobserved"
)

// EffortOrUnknown accepts only the cross-harness effort vocabulary and
// EffortInherit. An empty value is EffortUnobserved; anything else is
// "unknown", so a malformed value never reaches the log.
func EffortOrUnknown(candidate string) string {
	switch {
	case effortPattern.MatchString(candidate), candidate == EffortInherit:
		return candidate
	case candidate == "":
		return EffortUnobserved
	}
	return "unknown"
}

// HashSessionID avoids persisting a harness-provided session identifier.
func HashSessionID(sessionID string) string {
	if !sessionIDPattern.MatchString(sessionID) {
		return ""
	}
	sum := sha256.Sum256([]byte("harness-downshift-session-v1:" + sessionID))
	return hex.EncodeToString(sum[:])
}

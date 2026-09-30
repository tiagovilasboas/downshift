// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1
// Commercial use requires a licence — see LICENSE for terms.

// Package lifecycleobserver evaluates synthetic Codex lifecycle-hook fixtures.
// It has no hook registration, stdin reader, transcript access, persistence,
// provider client, or network adapter.
package lifecycleobserver

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"time"
)

const (
	DefaultEnabled = false
	SchemaVersion  = "harness-downshift.observer.v1"
)

// Event contains only documented Codex lifecycle-hook correlation fields.
// Transcript fields and assistant messages are intentionally absent.
type Event struct {
	HookEventName string
	SessionID     string
	TurnID        string
	ToolName      string
	AgentID       string
	AgentType     string
}

// Config requires explicit enablement and caller-owned ephemeral HMAC material.
// The observer never creates, writes, or persists a key.
type Config struct {
	Enabled    bool
	HMACKey    []byte
	ObserverID string
	Now        func() time.Time
}

// Observer is a local, in-memory fixture evaluator.
type Observer struct {
	enabled    bool
	hmacKey    []byte
	observerID string
	now        func() time.Time
}

// New returns an inert observer unless all opt-in prerequisites are supplied.
func New(config Config) Observer {
	if !config.Enabled || len(config.HMACKey) < 16 || config.ObserverID == "" {
		return Observer{}
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	return Observer{enabled: true, hmacKey: append([]byte(nil), config.HMACKey...), observerID: config.ObserverID, now: now}
}

// Observation is the sole POC output schema. It is prompt-free and carries no
// raw session, turn, agent, transcript, model, or reasoning-effort identifier.
// EvidenceState is lifecycle evidence, never executor acknowledgement.
type Observation struct {
	SchemaVersion    string `json:"schema_version"`
	EvidenceState    string `json:"evidence_state"`
	ObservedAt       string `json:"observed_at"`
	Lifecycle        string `json:"lifecycle"`
	ObserverIDHash   string `json:"observer_id_hash"`
	SubjectIDHash    string `json:"subject_id_hash"`
	AssociationState string `json:"association_state"`
	CorrelationID    string `json:"correlation_id,omitempty"`
}

// Observe evaluates only supplied fixtures. It uses session_id, turn_id,
// agent_id, and agent_type in memory, hashes them before output, and never
// guesses a correlation ID. Codex's documented lifecycle fields expose no
// opaque routing correlation ID, so even a completed lifecycle is unavailable
// for routing correlation rather than falsely matched.
func (observer Observer) Observe(events []Event) []Observation {
	if !observer.enabled {
		return nil
	}

	preToolUses := make(map[key][]Event)
	starts := make(map[key][]Event)
	stops := make(map[key][]Event)
	for _, event := range events {
		key, ok := eventKey(event)
		if !ok {
			continue
		}
		switch event.HookEventName {
		case "PreToolUse":
			if isSpawnTool(event.ToolName) {
				preToolUses[key] = append(preToolUses[key], event)
			}
		case "SubagentStart":
			if event.AgentID != "" && event.AgentType != "" {
				starts[key] = append(starts[key], event)
			}
		case "SubagentStop":
			if event.AgentID != "" && event.AgentType != "" {
				stops[key] = append(stops[key], event)
			}
		}
	}

	keys := make([]key, 0, len(preToolUses))
	for key := range preToolUses {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].sessionID == keys[j].sessionID {
			return keys[i].turnID < keys[j].turnID
		}
		return keys[i].sessionID < keys[j].sessionID
	})

	observations := make([]Observation, 0, len(keys))
	for _, key := range keys {
		observations = append(observations, observer.observeKey(key, preToolUses[key], starts[key], stops[key]))
	}
	return observations
}

type key struct {
	sessionID string
	turnID    string
}

func eventKey(event Event) (key, bool) {
	if event.SessionID == "" || event.TurnID == "" {
		return key{}, false
	}
	return key{sessionID: event.SessionID, turnID: event.TurnID}, true
}

func isSpawnTool(toolName string) bool {
	lower := strings.ToLower(toolName)
	return lower == "agent" || lower == "task" || strings.HasSuffix(lower, "spawn_agent")
}

func (observer Observer) observeKey(key key, preToolUses, starts, stops []Event) Observation {
	associationState := "unavailable"
	lifecycle := "needs_attention"
	subject := key.sessionID + "\x00" + key.turnID
	if len(preToolUses) > 1 || len(starts) > 1 {
		associationState = "ambiguous"
	} else if len(preToolUses) == 1 && len(starts) == 1 {
		start := starts[0]
		subject += "\x00" + start.AgentID + "\x00" + start.AgentType
		matchingStops := 0
		for _, stop := range stops {
			if stop.AgentID == start.AgentID && stop.AgentType == start.AgentType {
				matchingStops++
			}
		}
		switch matchingStops {
		case 1:
			lifecycle = "completed"
		case 0:
			associationState = "unavailable"
		default:
			associationState = "ambiguous"
		}
	}

	return Observation{
		SchemaVersion:    SchemaVersion,
		EvidenceState:    "observed",
		ObservedAt:       observer.now().UTC().Format(time.RFC3339),
		Lifecycle:        lifecycle,
		ObserverIDHash:   observer.hmac(observer.observerID),
		SubjectIDHash:    observer.hmac(subject),
		AssociationState: associationState,
	}
}

func (observer Observer) hmac(value string) string {
	mac := hmac.New(sha256.New, observer.hmacKey)
	_, _ = mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}

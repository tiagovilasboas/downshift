// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package quota

import (
	"encoding/json"
	"errors"
	"time"
)

// Native sources are deliberately distinct: context-window utilization and
// purchased-credit balance are not subscription quota and are never decoded.
func ParseNative(source string, data []byte, observed time.Time, ttl time.Duration) (Snapshot, error) {
	s := Snapshot{Version: 1, Source: source, ObservedAt: observed, ExpiresAt: observed.Add(ttl)}
	switch source {
	case "claude-statusline":
		s.Harness = "claude-code"
		var payload struct {
			Limits map[string]struct {
				Used  *float64 `json:"used_percentage"`
				Reset int64    `json:"resets_at"`
			} `json:"rate_limits"`
		}
		if json.Unmarshal(data, &payload) != nil || payload.Limits == nil {
			return s, errors.New("Claude statusline has no rate_limits")
		}
		for _, id := range []string{"five_hour", "seven_day"} {
			v := payload.Limits[id]
			s.Windows = append(s.Windows, window(id, "harness", nil, v.Used, v.Reset))
		}
	case "codex-usage":
		s.Harness = "codex"
		var payload struct {
			Limits map[string]struct {
				Model     string       `json:"normalModelSlug"`
				Primary   *codexWindow `json:"primary"`
				Secondary *codexWindow `json:"secondary"`
			} `json:"rateLimitsByLimitId"`
		}
		if json.Unmarshal(data, &payload) != nil || payload.Limits == nil {
			return s, errors.New("Codex usage has no rateLimitsByLimitId")
		}
		found := false
		for id, v := range payload.Limits {
			scope, ids := "models", []string{v.Model}
			if id == "codex" {
				scope, ids, found = "harness", nil, true
			} else if v.Model == "" {
				continue
			}
			if id != "codex" && v.Primary == nil && v.Secondary == nil {
				s.Windows = append(s.Windows, window(id+"/unknown", scope, ids, nil, 0))
			}
			for i, v := range []*codexWindow{v.Primary, v.Secondary} {
				wid := id + "/primary"
				if i == 1 {
					wid = id + "/secondary"
				}
				if v == nil {
					if id != "codex" {
						continue
					}
					s.Windows = append(s.Windows, window(wid, scope, ids, nil, 0))
					continue
				}
				s.Windows = append(s.Windows, window(wid, scope, ids, v.Used, v.Reset))
			}
		}
		if !found {
			return s, errors.New("Codex usage missing shared codex limits")
		}
	case "cursor-usage":
		s.Harness = "cursor"
		var err error
		data, err = canonicalCursorJSON(data)
		if err != nil {
			return s, err
		}
		var payload struct {
			Plan struct {
				Auto *float64 `json:"auto_percent_used"`
				API  *float64 `json:"api_percent_used"`
			} `json:"plan_usage"`
			AutoModels *[]string  `json:"auto_bucket_models"`
			Models     []string   `json:"available_models"`
			Complete   bool       `json:"available_models_complete"`
			End        int64      `json:"billing_cycle_end"`
			ObservedAt *time.Time `json:"observed_at"`
		}
		if json.Unmarshal(data, &payload) != nil {
			return s, errors.New("invalid Cursor usage payload")
		}
		if payload.ObservedAt != nil {
			s.ObservedAt = *payload.ObservedAt
			s.ExpiresAt = s.ObservedAt.Add(ttl)
			observed = s.ObservedAt
		}
		// Without provider membership, percentages cannot be assigned to IDs.
		if payload.AutoModels != nil {
			reset := payload.End
			if reset > 1_000_000_000_000 {
				reset /= 1000
			}
			// Refuse ambiguous epoch units rather than treating a far-future
			// or ancient timestamp as an active billing cycle.
			if reset < observed.Add(-366*24*time.Hour).Unix() || reset > observed.Add(366*24*time.Hour).Unix() {
				reset = 0
			}
			if len(*payload.AutoModels) > 0 {
				s.Windows = append(s.Windows, window("cursor-models", "models", *payload.AutoModels, payload.Plan.Auto, reset))
			}
			if payload.Complete {
				auto := map[string]bool{}
				for _, id := range *payload.AutoModels {
					auto[id] = true
				}
				var others []string
				for _, id := range payload.Models {
					if id != "" && !auto[id] {
						others = append(others, id)
					}
				}
				if len(others) > 0 {
					s.Windows = append(s.Windows, window("other-models", "models", others, payload.Plan.API, reset))
				}
			}
		}
	default:
		return s, errors.New("unsupported native quota source")
	}
	return s, s.Validate()
}

type codexWindow struct {
	Used  *float64 `json:"usedPercent"`
	Reset int64    `json:"resetsAt"`
}

func window(id, scope string, models []string, used *float64, reset int64) Window {
	w := Window{ID: id, Scope: scope, ModelIDs: models, UsedPercent: used}
	if reset > 0 {
		w.ResetsAt = time.Unix(reset, 0).UTC()
	}
	return w
}

// Cursor protobuf clients expose camelCase while exported local evidence may
// use protobuf snake_case. Only the verified fields are canonicalized.
func canonicalCursorJSON(data []byte) ([]byte, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil || raw == nil {
		return nil, errors.New("invalid Cursor usage payload")
	}
	alias := func(m map[string]json.RawMessage, snake, camel string) error {
		if v, exists := m[camel]; exists {
			if _, duplicate := m[snake]; duplicate {
				return errors.New("conflicting Cursor quota aliases")
			}
			m[snake] = v
			delete(m, camel)
		}
		return nil
	}
	for _, pair := range [][2]string{{"plan_usage", "planUsage"}, {"auto_bucket_models", "autoBucketModels"}, {"billing_cycle_end", "billingCycleEnd"}, {"available_models", "availableModels"}, {"available_models_complete", "availableModelsComplete"}, {"observed_at", "observedAt"}} {
		if err := alias(raw, pair[0], pair[1]); err != nil {
			return nil, err
		}
	}
	if v, ok := raw["plan_usage"]; ok {
		var plan map[string]json.RawMessage
		if err := json.Unmarshal(v, &plan); err != nil {
			return nil, errors.New("invalid Cursor plan_usage")
		}
		for _, pair := range [][2]string{{"auto_percent_used", "autoPercentUsed"}, {"api_percent_used", "apiPercentUsed"}} {
			if err := alias(plan, pair[0], pair[1]); err != nil {
				return nil, err
			}
		}
		b, err := json.Marshal(plan)
		if err != nil {
			return nil, err
		}
		raw["plan_usage"] = b
	}
	return json.Marshal(raw)
}

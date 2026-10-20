// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"
)

// OpenRouterEnv opts in to the OpenRouter source. Off by default: discovery
// makes no new network call unless the operator sets it to "1".
const OpenRouterEnv = "DOWNSHIFT_DISCOVERY_OPENROUTER"

// openRouterDefaultURL is OpenRouter's public model list. It needs no key.
const openRouterDefaultURL = "https://openrouter.ai/api/v1/models"

// maxOpenRouterBody bounds the response body. The full public list is far
// below this; anything larger is treated as unknown, not truncated.
const maxOpenRouterBody = 8 << 20

// OpenRouterAPI lists the public OpenRouter model catalogue. The harness is
// a field rather than a constant because the same listing can serve more than
// one harness that routes through OpenRouter; it defaults to "opencode".
//
// Prices are reported, never used to rank or to choose a model: tier comes
// only from the catalog or a provider descriptor, not from price.
type OpenRouterAPI struct {
	HarnessID string       // catalog harness id; default "opencode"
	URL       string       // default https://openrouter.ai/api/v1/models
	Client    *http.Client // default: no redirects, no key, no cookies
}

func (o OpenRouterAPI) Harness() string {
	if o.HarnessID == "" {
		return "opencode"
	}
	return o.HarnessID
}

func (OpenRouterAPI) Name() string { return "OpenRouter public models" }

func (o OpenRouterAPI) Discover(ctx context.Context) ([]Model, error) {
	target := o.URL
	if target == "" {
		target = openRouterDefaultURL
	}
	u, err := url.Parse(target)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return nil, errors.New("openrouter: only an https URL is allowed")
	}
	// Redirects are refused whatever client is supplied: the listing is one
	// fixed host, and a redirect is never followed.
	base := http.Client{}
	if o.Client != nil {
		base = *o.Client
	}
	base.CheckRedirect = func(*http.Request, []*http.Request) error {
		return errors.New("openrouter: redirects are refused")
	}
	client := &base
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "downshift-discovery (+https://github.com/tiagovilasboas/downshift)")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openrouter: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d from OpenRouter models", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxOpenRouterBody+1))
	if err != nil {
		return nil, fmt.Errorf("openrouter read: %w", err)
	}
	if len(body) > maxOpenRouterBody {
		return nil, errors.New("openrouter: response too large")
	}
	return parseOpenRouterList(body)
}

// parseOpenRouterList reads {"data":[{id,name,created,pricing{prompt,completion}}]}.
// Prices are strings of USD per token; they become USD per million tokens.
func parseOpenRouterList(data []byte) ([]Model, error) {
	var env struct {
		Data []struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Created int64  `json:"created"`
			Pricing struct {
				Prompt     string `json:"prompt"`
				Completion string `json:"completion"`
			} `json:"pricing"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("openrouter list: %w", err)
	}
	var out []Model
	for _, m := range env.Data {
		if m.ID == "" {
			continue
		}
		model := Model{
			ID:      m.ID,
			Display: m.Name,
			InputM:  perMillion(m.Pricing.Prompt),
			OutputM: perMillion(m.Pricing.Completion),
		}
		if m.Created > 0 {
			model.CreatedAt = time.Unix(m.Created, 0).UTC()
		}
		out = append(out, model)
	}
	if len(out) == 0 {
		return nil, errors.New("openrouter list: no models")
	}
	return out, nil
}

// perMillion converts a per-token USD price string to USD per million tokens.
// A missing, negative (OpenRouter's "dynamic" marker), non-numeric or
// non-finite price is unknown and returns nil.
func perMillion(s string) *float64 {
	if s == "" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v < 0 || math.IsNaN(v) || math.IsInf(v, 0) {
		return nil
	}
	m := v * 1e6
	return &m
}

// SnapshotPrice returns the OpenRouter list price of model from a discovery
// cache, in USD per million input and output tokens. Only entries fetched from
// the OpenRouter public list count; a model without both prices is not priced.
// It only reads the cache: no network call.
func SnapshotPrice(f File, model string) (inM, outM float64, ok bool) {
	for _, e := range f.Harnesses {
		if e.Source != (OpenRouterAPI{}).Name() {
			continue
		}
		for _, m := range e.Models {
			if m.ID == model && m.InputM != nil && m.OutputM != nil {
				return *m.InputM, *m.OutputM, true
			}
		}
	}
	return 0, 0, false
}

// openRouterEnabled reports whether the operator opted in.
func openRouterEnabled() bool {
	return os.Getenv(OpenRouterEnv) == "1"
}

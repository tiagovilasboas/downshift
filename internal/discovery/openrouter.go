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

// OpenRouterAPI lists OpenRouter's public model catalogue as a discovery
// source. It owns the fetch and parse of that one listing and names the source
// the discovery cache stores.
// The harness is a field rather than a constant because the same listing can
// serve more than one harness that routes through the gateway; it defaults to
// "opencode".
//
// Prices are reported, never used to rank or to choose a model: tier comes
// only from the catalog or a provider descriptor, not from price.
type OpenRouterAPI struct {
	HarnessID string       // catalog harness id; default "opencode"
	URL       string       // default: the adapter's public list URL
	Client    *http.Client // default: no redirects, no key, no cookies
}

func (o OpenRouterAPI) Harness() string {
	if o.HarnessID == "" {
		return "opencode"
	}
	return o.HarnessID
}

// Name is the source label stored in the discovery cache. It is a cache-format
// value: changing it orphans existing snapshots.
func (OpenRouterAPI) Name() string { return "OpenRouter public models" }

func (o OpenRouterAPI) Discover(ctx context.Context) ([]Model, error) {
	return fetchOpenRouterList(ctx, o.URL, o.Client)
}

// openRouterEnabled reports whether the operator opted in.
func openRouterEnabled() bool {
	return os.Getenv(OpenRouterEnv) == "1"
}

const (
	openRouterDefaultURL = "https://openrouter.ai/api/v1/models"
	openRouterMaxBody    = 8 << 20
)

// fetchOpenRouterList reads the public list. A zero URL means the public
// endpoint and a nil client never follows redirects. Redirects are refused
// whatever client is supplied: the listing is one fixed host.
func fetchOpenRouterList(ctx context.Context, rawURL string, client *http.Client) ([]Model, error) {
	target := rawURL
	if target == "" {
		target = openRouterDefaultURL
	}
	u, err := url.Parse(target)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return nil, errors.New("openrouter: only an https URL is allowed")
	}
	base := http.Client{}
	if client != nil {
		base = *client
	}
	base.CheckRedirect = func(*http.Request, []*http.Request) error {
		return errors.New("openrouter: redirects are refused")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "downshift-discovery (+https://github.com/tiagovilasboas/downshift)")
	resp, err := base.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openrouter: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d from OpenRouter models", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, openRouterMaxBody+1))
	if err != nil {
		return nil, fmt.Errorf("openrouter read: %w", err)
	}
	if len(body) > openRouterMaxBody {
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
			InputM:  openRouterPerMillion(m.Pricing.Prompt),
			OutputM: openRouterPerMillion(m.Pricing.Completion),
			Gateway: true,
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

// openRouterPerMillion converts a per-token USD price string to USD per million
// tokens. A missing, negative (OpenRouter's "dynamic" marker), non-numeric or
// non-finite price is unknown and returns nil.
func openRouterPerMillion(s string) *float64 {
	if s == "" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	// Scale first: a finite price near the float limit overflows to +Inf here.
	m := v * 1e6
	if math.IsNaN(m) || math.IsInf(m, 0) || m < 0 {
		return nil
	}
	return &m
}

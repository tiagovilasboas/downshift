// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package openrouter is the first gateway adapter: OpenRouter's public model
// list and its "vendor/model" id scheme. It holds the OpenRouter knowledge the
// shared layers no longer name. The composition root registers it.
package openrouter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift/internal/gateway"
)

// Name is the registry key and the catalog "provider" value of this gateway.
const Name = "openrouter"

// defaultURL is OpenRouter's public model list. It needs no key.
const defaultURL = "https://openrouter.ai/api/v1/models"

// maxBody bounds the response body. The full public list is far below this;
// anything larger is treated as unknown, not truncated.
const maxBody = 8 << 20

// Snapshot returns the list price of a canonical model in USD per million
// input and output tokens. It only reads what the composition root supplies;
// the adapter does no I/O for prices. A nil Snapshot prices nothing.
type Snapshot func(model string) (inPerM, outPerM float64, ok bool)

// Gateway implements gateway.Gateway for OpenRouter. The zero URL and nil
// Client mean the public endpoint and a client that never follows redirects.
type Gateway struct {
	Prices Snapshot
	URL    string
	Client *http.Client
}

// New returns the OpenRouter gateway that prices from snapshot.
func New(snapshot Snapshot) Gateway {
	return Gateway{Prices: snapshot}
}

// Name implements gateway.Gateway.
func (Gateway) Name() string { return Name }

// NormalizeID accepts OpenRouter's "vendor/model" form: exactly one slash with
// both parts present. The id is already canonical, so it is returned as is.
func (Gateway) NormalizeID(raw string) (string, bool) {
	vendor, model, ok := strings.Cut(raw, "/")
	if !ok || vendor == "" || model == "" || strings.Contains(model, "/") {
		return "", false
	}
	return raw, true
}

// Price implements gateway.Gateway from the supplied snapshot.
func (g Gateway) Price(canonical string) (float64, float64, bool) {
	if g.Prices == nil {
		return 0, 0, false
	}
	return g.Prices(canonical)
}

// Privacy implements gateway.Gateway. No zero-retention evidence is read yet,
// so every model is unknown.
func (Gateway) Privacy(string) gateway.PrivacyInfo {
	return gateway.PrivacyInfo{Retention: gateway.RetentionUnknown, Source: "openrouter: no zero-retention evidence"}
}

// ListModels implements gateway.Gateway by reading the public list. Redirects
// are refused whatever client is supplied: the listing is one fixed host.
func (g Gateway) ListModels(ctx context.Context) ([]gateway.ModelInfo, error) {
	target := g.URL
	if target == "" {
		target = defaultURL
	}
	u, err := url.Parse(target)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return nil, errors.New("openrouter: only an https URL is allowed")
	}
	base := http.Client{}
	if g.Client != nil {
		base = *g.Client
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
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return nil, fmt.Errorf("openrouter read: %w", err)
	}
	if len(body) > maxBody {
		return nil, errors.New("openrouter: response too large")
	}
	return parseList(body)
}

// parseList reads {"data":[{id,name,created,pricing{prompt,completion}}]}.
// Prices are strings of USD per token; they become USD per million tokens.
func parseList(data []byte) ([]gateway.ModelInfo, error) {
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
	var out []gateway.ModelInfo
	for _, m := range env.Data {
		if m.ID == "" {
			continue
		}
		model := gateway.ModelInfo{
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

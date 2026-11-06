// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// AnthropicAPI lists models through the Anthropic Models API. It needs an API
// key (ANTHROPIC_API_KEY); subscription-only Claude Code sessions have no
// listing endpoint and are skipped. The API reports created_at, which is how
// the newest model of a family is chosen without parsing names.
type AnthropicAPI struct {
	URL    string // default https://api.anthropic.com/v1/models
	Key    string // default $ANTHROPIC_API_KEY
	Client *http.Client
}

func (AnthropicAPI) Harness() string { return "claude-code" }
func (AnthropicAPI) Name() string    { return "Anthropic Models API" }

func (a AnthropicAPI) Discover(ctx context.Context) ([]Model, error) {
	key := a.Key
	if key == "" {
		key = os.Getenv("ANTHROPIC_API_KEY")
	}
	if key == "" {
		return nil, errors.New("no ANTHROPIC_API_KEY (subscription sessions cannot be listed)")
	}
	url := a.URL
	if url == "" {
		url = "https://api.anthropic.com/v1/models?limit=1000"
	}
	client := a.Client
	if client == nil {
		// Never follow a redirect: Go would forward x-api-key to the new host.
		client = &http.Client{
			Timeout:       10 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", key)
	req.Header.Set("anthropic-version", "2023-06-01")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d from Anthropic Models API", resp.StatusCode)
	}
	var env struct {
		Data []struct {
			ID      string    `json:"id"`
			Display string    `json:"display_name"`
			Created time.Time `json:"created_at"`
		} `json:"data"`
	}
	dec := json.NewDecoder(io.LimitReader(resp.Body, maxOutput))
	if err := dec.Decode(&env); err != nil {
		return nil, fmt.Errorf("anthropic list: %w", err)
	}
	var out []Model
	for _, m := range env.Data {
		if m.ID != "" {
			out = append(out, Model{ID: m.ID, Display: m.Display, CreatedAt: m.Created})
		}
	}
	if len(out) == 0 {
		return nil, errors.New("anthropic list: no models")
	}
	return out, nil
}

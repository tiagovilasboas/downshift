// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"context"
	"net/http"
	"os"

	"github.com/tiagovilasboas/downshift/internal/gateway/openrouter"
)

// OpenRouterEnv opts in to the OpenRouter source. Off by default: discovery
// makes no new network call unless the operator sets it to "1".
const OpenRouterEnv = "DOWNSHIFT_DISCOVERY_OPENROUTER"

// OpenRouterAPI lists a gateway's public model catalogue as a discovery
// source. The fetch and parse live in the gateway adapter; this type only maps
// the listing into the discovery cache and names the source the cache stores.
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
	listed, err := openrouter.Gateway{URL: o.URL, Client: o.Client}.ListModels(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Model, 0, len(listed))
	for _, m := range listed {
		out = append(out, Model{
			ID:        m.ID,
			Display:   m.Display,
			CreatedAt: m.CreatedAt,
			InputM:    m.InputM,
			OutputM:   m.OutputM,
		})
	}
	return out, nil
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

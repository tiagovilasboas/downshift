// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/gateway"
)

type fakeGateway struct {
	name     string
	endpoint string
	healthy  bool
	detail   string
}

func (f *fakeGateway) Name() string            { return f.name }
func (f *fakeGateway) DefaultEndpoint() string { return f.endpoint }
func (f *fakeGateway) NormalizeID(raw string) (string, bool) {
	return raw, true
}
func (f *fakeGateway) CheckHealth(ctx context.Context, endpoint string) (bool, string) {
	return f.healthy, f.detail
}

func TestCheckGateways_Nil(t *testing.T) {
	rep := &doctorReport{}
	checkGateways(rep, doctorEnv{})
	if len(rep.Checks) != 0 {
		t.Fatalf("expected 0 checks for nil gateways, got %d", len(rep.Checks))
	}
}

func TestCheckGateways_Active(t *testing.T) {
	rep := &doctorReport{}
	env := doctorEnv{
		Gateways: func() []gateway.Gateway {
			return []gateway.Gateway{
				&fakeGateway{
					name:     "omniroute",
					endpoint: "http://localhost:20128/v1",
					healthy:  true,
					detail:   "ok",
				},
			}
		},
	}
	checkGateways(rep, env)
	if len(rep.Checks) != 1 {
		t.Fatalf("expected 1 check for active gateway, got %d", len(rep.Checks))
	}
	c := rep.Checks[0]
	if c.ID != "gateway" || c.Status != doctorOK {
		t.Errorf("unexpected check: %+v", c)
	}
	if !strings.Contains(c.Detail, "omniroute active at http://localhost:20128/v1") {
		t.Errorf("unexpected detail: %q", c.Detail)
	}
	if !strings.Contains(c.Detail, "prefix: omniroute/<provider>/<model>") {
		t.Errorf("missing prefix in detail: %q", c.Detail)
	}
}

func TestCheckGateways_OfflineQuiet(t *testing.T) {
	rep := &doctorReport{}
	env := doctorEnv{
		Verbose: false,
		Gateways: func() []gateway.Gateway {
			return []gateway.Gateway{
				&fakeGateway{
					name:     "9router",
					endpoint: "http://localhost:3000",
					healthy:  false,
					detail:   "connection refused",
				},
			}
		},
	}
	checkGateways(rep, env)
	if len(rep.Checks) != 0 {
		t.Fatalf("expected 0 checks for offline gateway in quiet mode, got %d", len(rep.Checks))
	}
}

func TestCheckGateways_OfflineVerbose(t *testing.T) {
	rep := &doctorReport{}
	env := doctorEnv{
		Verbose: true,
		Gateways: func() []gateway.Gateway {
			return []gateway.Gateway{
				&fakeGateway{
					name:     "9router",
					endpoint: "http://localhost:3000",
					healthy:  false,
					detail:   "connection refused",
				},
			}
		},
	}
	checkGateways(rep, env)
	if len(rep.Checks) != 1 {
		t.Fatalf("expected 1 check for offline gateway in verbose mode, got %d", len(rep.Checks))
	}
	c := rep.Checks[0]
	if c.ID != "gateway" || c.Status != doctorOK {
		t.Errorf("unexpected check: %+v", c)
	}
	if !strings.Contains(c.Detail, "9router not detected") {
		t.Errorf("unexpected detail: %q", c.Detail)
	}
}

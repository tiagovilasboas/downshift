// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/catalog"
)

func pricesCheck(t *testing.T, ageDays int) doctorCheck {
	t.Helper()
	cat := catalog.Load()
	asOf, err := time.Parse("2006-01-02", cat.PricesAsOf())
	if err != nil {
		t.Fatalf("embedded prices_as_of: %v", err)
	}
	rep := &doctorReport{}
	checkPrices(rep, doctorEnv{Cat: cat, Now: func() time.Time { return asOf.Add(time.Duration(ageDays) * 24 * time.Hour) }})
	if len(rep.Checks) != 1 || rep.Checks[0].ID != "prices" {
		t.Fatalf("want one prices check, got %+v", rep.Checks)
	}
	return rep.Checks[0]
}

func TestDoctorPricesFreshIsOK(t *testing.T) {
	c := pricesCheck(t, 10)
	if c.Status != doctorOK || !strings.Contains(c.Detail, "list prices as of") {
		t.Errorf("fresh prices: %+v", c)
	}
}

func TestDoctorPricesOlderThan30DaysWarns(t *testing.T) {
	c := pricesCheck(t, 31)
	if c.Status != doctorWarn || c.Fix == "" || !strings.Contains(c.Detail, "31 days old") ||
		strings.Contains(c.Detail, "estimate") || !strings.Contains(c.Detail, "routing") {
		t.Errorf("stale prices: %+v", c)
	}
}

func TestDoctorPricesBoundaryIs30DaysInclusive(t *testing.T) {
	if c := pricesCheck(t, 30); c.Status != doctorOK {
		t.Errorf("30 days old must still be ok: %+v", c)
	}
}

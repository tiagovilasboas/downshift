// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"
)

// doctor stopped checking the price table: prices do not order routing and
// never feed token savings. No check carries the "prices" id and no detail
// mentions a price.
func TestDoctor_NoPricesCheck(t *testing.T) {
	f := newDoctorFixture(t)
	rep := buildDoctorReport(f.env)
	for _, c := range rep.Checks {
		if c.ID == "prices" {
			t.Fatalf("doctor must not run a prices check, got %+v", c)
		}
		// The removed checkPrices wrote these exact phrases; nothing else should.
		for _, phrase := range []string{"list prices as of", "price table", "prices_as_of"} {
			if strings.Contains(c.Detail, phrase) {
				t.Fatalf("a doctor check still reports a price table: %+v", c)
			}
		}
	}
}

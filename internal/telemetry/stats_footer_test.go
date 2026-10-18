// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry_test

import (
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// stats estimates at list price; report measures with a control group. The
// headline says so and points at report, in the short and the detailed view.
func TestPrintStats_PointsAtReportForTheMeasuredNumber(t *testing.T) {
	for _, detail := range []bool{false, true} {
		var b strings.Builder
		telemetry.PrintStats(nil, telemetry.StatsOptions{Days: 30, Detail: detail}, &b)
		out := b.String()
		if !strings.Contains(out, "not an invoice") || !strings.Contains(out, "downshift report") {
			t.Errorf("detail=%v: no pointer to report in:\n%s", detail, out)
		}
		if strings.Count(out, "downshift report") != 1 {
			t.Errorf("detail=%v: the pointer should appear once:\n%s", detail, out)
		}
	}
}

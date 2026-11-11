// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor_test

import (
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/compressor"
)

func TestShortenPass_FamilyTimingKeptForms(t *testing.T) {
	for _, tc := range []struct {
		name, in, want string
	}{
		{"rspec", "PASS rspec: 12 examples, 0 failures (0.45s)\n", "PASS rspec: 12 examples, 0 failures\n"},
		{"rspec pending", "PASS rspec: 12 examples, 0 failures, 2 pending (3s)\n", "PASS rspec: 12 examples, 0 failures, 2 pending\n"},
		{"dotnet simple", "PASS dotnet test: 7 tests (1.2s)\n", "PASS dotnet test: 7 tests\n"},
		{"dotnet full", "PASS dotnet test: 40 passed / 40 total (2.5s)\n", "PASS dotnet test: 40 passed / 40 total\n"},
		{"dotnet skipped", "PASS dotnet test: 38 passed, 2 skipped / 40 total (2s)\n", "PASS dotnet test: 38 passed, 2 skipped / 40 total\n"},
		{"cypress", "Cypress: PASS (25 tests: 24 passing, 1 pending, 0 skipped; 8 seconds)\n", "Cypress: PASS (25 tests: 24 passing, 1 pending, 0 skipped)\n"},
		{"cypress spec", "Cypress: PASS (3 tests: 3 passing, 0 pending, 0 skipped; spec a.cy.js; 1 second)\n", "Cypress: PASS (3 tests: 3 passing, 0 pending, 0 skipped; spec a.cy.js)\n"},
		{"cypress keeps later lines", "Cypress: PASS (3 tests: 3 passing, 0 pending, 0 skipped; 1 second)\nnote\n", "Cypress: PASS (3 tests: 3 passing, 0 pending, 0 skipped)\nnote\n"},
		{"go verbose", "Go test -v: PASS (5 passed, 0 failed, 1 skipped) in 0.42s\nPackage: x/a\n", "Go test -v: PASS (5 passed, 0 failed, 1 skipped)\nPackage: x/a\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := compressor.ShortenPass([]byte(tc.in))
			if !ok || string(got) != tc.want {
				t.Fatalf("got %q (%v), want %q", got, ok, tc.want)
			}
			if again, ok := compressor.ShortenPass(got); ok || string(again) != tc.want {
				t.Fatalf("not idempotent: %q", again)
			}
		})
	}
}

func TestShortenPass_FamilyTimingStaysWhole(t *testing.T) {
	for name, in := range map[string]string{
		"rspec unknown duration":      "PASS rspec: 12 examples, 0 failures (?)\n",
		"rspec failures":              "FAIL rspec: 12 examples, 2 failures (0.45s)\n",
		"rspec extra text":            "PASS rspec: 12 examples, 0 failures (0.45s) extra\n",
		"dotnet unknown duration":     "PASS dotnet test: 7 tests (?)\n",
		"dotnet failed":               "FAIL dotnet test: 5 passed, 2 failed / 7 total (1s)\n",
		"cypress no counts":           "Cypress: PASS (25 tests: 24 passing; 8 seconds)\n",
		"cypress odd duration":        "Cypress: PASS (3 tests: 3 passing, 0 pending, 0 skipped; 00:08)\n",
		"cypress no duration":         "Cypress: PASS (3 tests: 3 passing, 0 pending, 0 skipped)\n",
		"go verbose cached":           "Go test -v: PASS (5 passed, 0 failed, 0 skipped) in (cached)\n",
		"go verbose failures":         "Go test -v: PASS (5 passed, 1 failed, 0 skipped) in 0.4s\n",
		"timing only in a later line": "Note\nPASS rspec: 12 examples, 0 failures (0.45s)\n",
	} {
		t.Run(name, func(t *testing.T) {
			got, ok := compressor.ShortenPass([]byte(in))
			if ok || string(got) != in {
				t.Fatalf("changed: %q (%v)", got, ok)
			}
		})
	}
}

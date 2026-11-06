// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package home

import (
	"errors"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/console/shared/command"
	"github.com/tiagovilasboas/downshift/internal/console/shared/theme"
)

func TestParseHealthOrdersFailsFirst(t *testing.T) {
	r := command.Result{Code: 1, Stdout: `{"checks":[{"id":"a","status":"ok"},{"id":"w","status":"warn","fix":"do w"},{"id":"f","status":"fail","detail":"broken"}]}`}
	h, err := parseHealth(r)
	if err != nil {
		t.Fatal(err)
	}
	if h.ok != 1 || h.warn != 1 || h.fail != 1 || h.issues[0].ID != "f" {
		t.Fatalf("health = %+v", h)
	}
	out := healthBlock(theme.New(), h, nil, 80)
	if !strings.Contains(out, "fix: do w") {
		t.Errorf("fix line missing:\n%s", out)
	}
}

func TestParseHealthErrors(t *testing.T) {
	if _, err := parseHealth(command.Result{Stdout: "not json"}); err == nil {
		t.Error("non-JSON output should be an error")
	}
	if _, err := parseHealth(command.Result{Err: errors.New("boom")}); err == nil {
		t.Error("start failure should be an error")
	}
}

func TestEmptySessionState(t *testing.T) {
	out := sessionBlock(theme.New(), command.Result{Code: 1}, 80)
	if !strings.Contains(out, "No subagent spawns") {
		t.Errorf("empty state missing: %q", out)
	}
}

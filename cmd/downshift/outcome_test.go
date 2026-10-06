// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestEvalOutcomeRecordRefusedInCI(t *testing.T) {
	t.Setenv("CI", "true")
	var out, errb bytes.Buffer
	code := runEvalOutcome([]string{"--record", "--tier=small", "--model=x", "--solver=true"}, &out, &errb)
	if code != 2 || !strings.Contains(errb.String(), "disabled when CI is set") {
		t.Fatalf("code=%d stderr=%q", code, errb.String())
	}
}

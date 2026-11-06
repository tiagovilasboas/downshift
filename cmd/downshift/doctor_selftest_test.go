// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"testing"
)

func TestSelfTestCompactor(t *testing.T) {
	if err := selfTestCompactor(); err != nil {
		t.Fatal(err)
	}
}

func TestSelfTestRequiresHarness(t *testing.T) {
	var out, errs bytes.Buffer
	if got := runDoctor([]string{"--self-test"}, &out, &errs, nil); got != 2 {
		t.Fatalf("exit %d: %s", got, errs.String())
	}
}

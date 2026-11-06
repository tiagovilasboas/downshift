// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"testing"
)

// Without a terminal on stdin and stdout the bare entry stays plain.
func TestWantsConsoleFalseWithoutTerminal(t *testing.T) {
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if wantsConsole(f, f) {
		t.Fatal("wantsConsole opened the console on a non-terminal")
	}
}

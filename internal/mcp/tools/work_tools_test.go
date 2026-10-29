// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package tools

import "testing"

// Write is the owner's switch: off, a write request is refused before anything
// starts, and read is always allowed.
func TestWantsWriteRefusesWriteUnlessTheOwnerEnablesIt(t *testing.T) {
	if w, err := wantsWrite("read", false); err != nil || w {
		t.Fatalf("read = %v, %v", w, err)
	}
	if _, err := wantsWrite("write", false); err == nil {
		t.Fatal("write must be refused while the owner has not enabled it")
	}
	if w, err := wantsWrite("write", true); err != nil || !w {
		t.Fatalf("write when enabled = %v, %v", w, err)
	}
	if _, err := wantsWrite("delete", true); err == nil {
		t.Fatal("an unknown mode must be refused")
	}
}

// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package setup

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// A junction needs no administrator rights. Go reports it as irregular, which
// linkedComponent treats like a symlink.
func TestLinkedComponentRefusesAJunction(t *testing.T) {
	home := t.TempDir()
	target := filepath.Join(home, "dotfiles", "opencode")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	junction := filepath.Join(home, "opencode")
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", junction, target).CombinedOutput(); err != nil {
		t.Skipf("cannot create a junction: %v: %s", err, out)
	}
	if got := linkedComponent(home, filepath.Join(junction, "plugins", "x")); got != junction {
		t.Fatalf("linkedComponent = %q, want the junction %q", got, junction)
	}
	if err := ErrLinkedPath(filepath.Join(junction, "x"), home); err == nil {
		t.Fatal("ErrLinkedPath did not refuse a path under a junction")
	}
}

// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/setup"
)

// An older harness setup still calls `downshift dci hook <harness>` on every
// prompt submit. In Claude Code a non-zero exit there blocks the prompt, so the
// removed command must stay silent and exit 0 for every harness.
func TestDCIRemoved_HookIsSilentAndExitsZero(t *testing.T) {
	for _, harness := range []string{"claude-code", "cursor", "codex", "kirocrew", "generic", ""} {
		var errOut bytes.Buffer
		args := []string{"hook"}
		if harness != "" {
			args = append(args, harness)
		}
		if code := runDCIRemoved(args, &errOut); code != 0 {
			t.Errorf("dci hook %q exit = %d, want 0", harness, code)
		}
		if errOut.Len() != 0 {
			t.Errorf("dci hook %q wrote %q, want nothing", harness, errOut.String())
		}
	}
}

func TestDCIRemoved_OtherFormsSayItIsGone(t *testing.T) {
	for _, args := range [][]string{nil, {"status"}, {"enable", "observe"}, {"inspect", "x"}} {
		var errOut bytes.Buffer
		if code := runDCIRemoved(args, &errOut); code != 0 {
			t.Errorf("dci %v exit = %d, want 0", args, code)
		}
		if !strings.Contains(errOut.String(), "removed") {
			t.Errorf("dci %v notice = %q, want it to say the feature was removed", args, errOut.String())
		}
	}
}

// Neither setup nor the plugin's hooks file may register the removed hook.
func TestNoTargetRegistersTheDCIHook(t *testing.T) {
	for _, st := range setup.Targets {
		for _, h := range st.Hooks {
			if strings.HasPrefix(h.Args, "dci") {
				t.Errorf("%s still registers %s %q", st.Name, h.Event, h.Args)
			}
		}
	}
	raw, err := os.ReadFile(filepath.Join("..", "..", "hooks", "hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "dci") {
		t.Errorf("hooks/hooks.json still registers the removed dci prompt hook")
	}
}

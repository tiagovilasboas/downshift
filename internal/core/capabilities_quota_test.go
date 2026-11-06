// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/quota"
)

func TestNativeCapsAgreeWithQuotaTable(t *testing.T) {
	for _, n := range core.AllHarnessCaps() {
		env, src, ok := quota.NativeSource(n.Harness)
		if ok != (n.Caps.NativeSessionEnv != "") || env != n.Caps.NativeSessionEnv || src != n.Caps.NativeUsageSource {
			t.Fatalf("%s: quota table and caps disagree", n.Harness)
		}
	}
	if !core.CodexCaps.TranscriptQuota || core.CursorCaps.TranscriptQuota {
		t.Fatal("transcript quota must be codex-only")
	}
}

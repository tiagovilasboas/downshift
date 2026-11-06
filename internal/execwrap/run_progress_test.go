// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"strings"
	"testing"
)

func TestRun_NpmInstallProgressFramesCollapse(t *testing.T) {
	fakeBin(t, "npm", "printf '%b' 'Resolving\\n"+strings.Repeat("Progress: resolved 1\\r", 80)+"Progress: resolved 80\\nDone\\n'")
	r, out, _ := run(t, []string{"npm", "install"}, true)
	if r.ExitCode != 0 || !strings.HasPrefix(out, "Resolving\nProgress: resolved 80\nDone\n") ||
		!strings.Contains(out, "[downshift compact:") {
		t.Fatalf("exit=%d out=%q", r.ExitCode, out)
	}
}

func TestRun_NpmInstallShrinkingFramesStayRaw(t *testing.T) {
	fakeBin(t, "npm", "printf 'abcdef\\rxy\\n'")
	_, out, _ := run(t, []string{"npm", "install"}, true)
	if out != "abcdef\rxy\n" {
		t.Fatalf("out=%q", out)
	}
}

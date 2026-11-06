// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"fmt"
	"strings"
	"testing"
)

// The fake binary replays a hand-written log; the real tools are never run.
func TestRun_WebAssetsCleanCompactsFailureStaysRaw(t *testing.T) {
	var b strings.Builder
	for i := 1; i <= 300; i++ {
		fmt.Fprintf(&b, "processed src/styles/part%03d.css in %dms\n", i, i%7)
	}
	clean := b.String()
	cases := []struct {
		bin  string
		argv []string
	}{
		{"stylelint", []string{"stylelint", "src/**/*.css"}},
		{"htmlhint", []string{"htmlhint", "dist/"}},
		{"html-validate", []string{"html-validate", "dist/index.html"}},
		{"tailwindcss", []string{"tailwindcss", "-i", "in.css", "-o", "out.css"}},
		{"sass", []string{"sass", "in.scss", "out.css"}},
		{"postcss", []string{"postcss", "in.css", "-o", "out.css"}},
	}
	for _, c := range cases {
		fakeBin(t, c.bin, "cat <<'EOF'\n"+clean+"EOF")
		r, out, _ := run(t, c.argv, true)
		if r.ExitCode != 0 || len(out) >= len(clean) || !strings.Contains(out, "lines elided]") {
			t.Fatalf("%s clean: exit=%d len=%d/%d", c.bin, r.ExitCode, len(out), len(clean))
		}
		fakeBin(t, c.bin, "cat <<'EOF'\n"+clean+"EOF\nexit 1")
		if r, out, _ := run(t, c.argv, true); r.ExitCode != 1 || out != clean {
			t.Fatalf("%s failed run: exit=%d changed=%v", c.bin, r.ExitCode, out != clean)
		}
	}
}

// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"strings"
	"testing"
)

func longLog(n int, tail string) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteString("building step\n")
		b.WriteString("step detail " + strings.Repeat("x", i%7) + string(rune('a'+i%26)) + "\n")
	}
	return b.String() + tail
}

func TestRun_GenericChainCompactsUnknownCleanOutput(t *testing.T) {
	raw := longLog(2000, "all done\n")
	fakeBin(t, "make", "cat <<'EOF'\n"+raw+"EOF")
	r, out, _ := run(t, []string{"make", "build"}, true)
	if r.ExitCode != 0 || !strings.Contains(out, "lines elided]") || len(out) >= len(raw) {
		t.Fatalf("exit=%d len=%d/%d out=%.200q", r.ExitCode, len(out), len(raw), out)
	}
	if !strings.Contains(out, "all done\n") || !strings.Contains(out, "[downshift compact:") {
		t.Fatalf("tail or marker missing: %.200q", out[len(out)-200:])
	}
}

func TestRun_GenericChainKeepsDangerRawWithExitCode(t *testing.T) {
	cases := map[string]string{
		"warning": longLog(2000, "warning: deprecated\n"),
		"error":   longLog(2000, "error: boom\n"),
		"fail":    longLog(2000, "1 FAILED\n"),
	}
	for name, raw := range cases {
		fakeBin(t, "make", "cat <<'EOF'\n"+raw+"EOF")
		r, out, _ := run(t, []string{"make", "build"}, true)
		if r.ExitCode != 0 || out != raw {
			t.Errorf("%s: exit=%d, output not byte for byte", name, r.ExitCode)
		}
	}
}

func TestRun_GenericChainKeepsNonZeroExitRaw(t *testing.T) {
	raw := longLog(2000, "done\n")
	fakeBin(t, "make", "cat <<'EOF'\n"+raw+"EOF\nexit 3")
	r, out, _ := run(t, []string{"make", "build"}, true)
	if r.ExitCode != 3 || out != raw {
		t.Fatalf("exit=%d, raw preserved=%v", r.ExitCode, out == raw)
	}
}

func TestRun_GenericChainSkipsGitFamilies(t *testing.T) {
	raw := longLog(2000, "done\n")
	fakeBin(t, "git", "cat <<'EOF'\n"+raw+"EOF")
	_, out, _ := run(t, []string{"git", "log"}, true)
	if out != raw {
		t.Fatal("git output must never go through the generic chain")
	}
}

func javaFrames(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteString("\tat com.acme.Svc" + strings.Repeat("x", i%5) + ".run(Svc.java:" + string(rune('0'+i%10)) + "1)\n")
	}
	return b.String()
}

func TestRun_GenericChainFoldsLongFrameRun(t *testing.T) {
	raw := "caller dump follows\n" + javaFrames(40) + "done\n"
	fakeBin(t, "make", "cat <<'EOF'\n"+raw+"EOF")
	r, out, _ := run(t, []string{"make", "build"}, true)
	if r.ExitCode != 0 || !strings.Contains(out, "[30 frames elided]") || len(out) >= len(raw) {
		t.Fatalf("exit=%d len=%d/%d out=%.300q", r.ExitCode, len(out), len(raw), out)
	}
}

func TestRun_GenericChainKeepsFrameRunRawWithErrorMessage(t *testing.T) {
	raw := "java.lang.IllegalStateException: boom\n" + javaFrames(40) + "done\n"
	fakeBin(t, "make", "cat <<'EOF'\n"+raw+"EOF")
	r, out, _ := run(t, []string{"make", "build"}, true)
	if r.ExitCode != 0 || out != raw {
		t.Fatalf("exit=%d, raw preserved=%v", r.ExitCode, out == raw)
	}
}

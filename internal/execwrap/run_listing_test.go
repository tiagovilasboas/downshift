// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/compressor"
)

// lsLongRaw is a long listing with a total line, dot entries and directories.
func lsLongRaw() string {
	var b strings.Builder
	b.WriteString("total 96\n")
	b.WriteString("drwxr-xr-x  12 tiago.boas  staff  384 Nov  4 10:00 .\n")
	b.WriteString("drwxr-xr-x   5 tiago.boas  staff  160 Nov  4 09:12 ..\n")
	for i := 0; i < 40; i++ {
		b.WriteString(fmt.Sprintf("-rw-r--r--   1 tiago.boas  staff  %4d Nov  4 10:00 module_%02d.go\n", 1000+i, i))
	}
	b.WriteString("drwxr-xr-x   9 tiago.boas  staff  288 Nov  4 10:00 compressor\n")
	return b.String()
}

// findRaw is a find listing of many paths under one tree.
func findRaw() string {
	var b strings.Builder
	for i := 0; i < 120; i++ {
		b.WriteString(fmt.Sprintf("internal/pipeline/stage%02d/handler_%03d.go\n", i%8, i))
	}
	return b.String()
}

// grepRaw is a grep -rn listing with many matches across files.
func grepRaw() string {
	var b strings.Builder
	for i := 0; i < 120; i++ {
		b.WriteString(fmt.Sprintf("internal/compressor/module_%02d.go:%d:\tfunc handler%d(input []byte) {\n", i%10, 10+i, i))
	}
	return b.String()
}

// ls -l / ls -la compacts by stripping permissions, links, owner and group.
func TestRun_LsLongFormCompacts(t *testing.T) {
	raw := lsLongRaw()
	fakeBin(t, "ls", "cat <<'EOF'\n"+raw+"EOF")
	r, out, _ := run(t, []string{"ls", "-la", "internal"}, true)
	if r.ExitCode != 0 || !r.Compacted || !strings.Contains(out, "module_00.go") || !strings.Contains(out, "compressor/") {
		t.Fatalf("exit=%d compacted=%v out=%.120q", r.ExitCode, r.Compacted, out)
	}
}

func TestRun_NonZeroExitStaysRaw(t *testing.T) {
	cases := []struct {
		name, argv0 string
		argv        []string
		raw         string
	}{
		{"ls", "ls", []string{"ls", "-la", "missing"}, lsLongRaw()},
		{"find", "find", []string{"find", "internal", "-name", "*.go"}, findRaw()},
		{"grep", "grep", []string{"grep", "-rn", "func", "internal"}, grepRaw()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakeBin(t, tc.argv0, "cat <<'EOF'\n"+tc.raw+"EOF\nexit 2")
			r, out, _ := run(t, tc.argv, true)
			if r.ExitCode != 2 || r.Compacted || out != tc.raw {
				t.Fatalf("exit=%d compacted=%v passthrough=%q, output changed", r.ExitCode, r.Compacted, r.Passthrough)
			}
		})
	}
}

func TestRun_FindCompactsCleanListingAsTree(t *testing.T) {
	raw := findRaw()
	fakeBin(t, "find", "cat <<'EOF'\n"+raw+"EOF")
	r, out, _ := run(t, []string{"find", "internal", "-name", "*.go"}, true)
	want := compressor.CompressCommandOutput([]byte(raw), compressor.FormatFind, 0).Output
	if r.ExitCode != 0 || !r.Compacted || out != string(want)+Marker(len(raw), len(want)) {
		t.Fatalf("exit=%d compacted=%v out=%.200q", r.ExitCode, r.Compacted, out)
	}
}

func TestRun_GrepCompactsCleanMatchesByFile(t *testing.T) {
	raw := grepRaw()
	fakeBin(t, "grep", "cat <<'EOF'\n"+raw+"EOF")
	r, out, _ := run(t, []string{"grep", "-rn", "func", "internal/compressor"}, true)
	want := compressor.CompressCommandOutput([]byte(raw), compressor.FormatGrep, 0).Output
	if r.ExitCode != 0 || !r.Compacted || out != string(want)+Marker(len(raw), len(want)) {
		t.Fatalf("exit=%d compacted=%v out=%.200q", r.ExitCode, r.Compacted, out)
	}
	if !strings.Contains(out, "internal/compressor/module_00.go\n") || !strings.Contains(out, " 10: func handler0(") {
		t.Fatalf("grouping or trimmed match missing: %.200q", out)
	}
}

func TestRun_FindActionsAreRefusedAndNeverRun(t *testing.T) {
	for _, action := range [][]string{
		{"-exec", "true", ";"},
		{"-execdir", "true", ";"},
		{"-ok", "true", ";"},
		{"-okdir", "true", ";"},
		{"-delete"},
		{"-fprint", "out.txt"},
		{"-fprint0", "out.txt"},
		{"-fprintf", "out.txt", "%p"},
		{"-fls", "out.txt"},
	} {
		t.Run(action[0], func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "ran")
			fakeBin(t, "find", "touch '"+marker+"'\necho listing")
			argv := append([]string{"find", "."}, action...)
			r, out, _ := run(t, argv, true)
			if r.ExitClass != ExitClassRefused || r.ExitCode != ExitCannotExecute || out != "" {
				t.Fatalf("exit=%d class=%q out=%q", r.ExitCode, r.ExitClass, out)
			}
			if _, err := os.Stat(marker); err == nil {
				t.Fatalf("refused find ran anyway")
			}
		})
	}
}

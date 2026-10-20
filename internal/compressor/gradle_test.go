// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"strings"
	"testing"
)

func TestCompressGradlePreserved(t *testing.T) {
	ok := "\nBUILD SUCCESSFUL in 2s\n2 actionable tasks: 1 executed, 1 up-to-date\n"
	head := "> Task :app:a UP-TO-DATE\n> Task :app:b\n"
	for name, in := range map[string]string{
		"no newline": strings.TrimSuffix(head+ok, "\n"),
		"crlf":       strings.ReplaceAll(head+ok, "\n", "\r\n"),
		"ansi":       "\x1b[1m" + head + ok,
		"kotlin e":   head + "e: file:///x.kt:1:1 error\n" + ok,
		"warning":    head + "warning: [deprecation] foo\n" + ok,
		"note":       head + "Note: Some input files use unchecked operations.\n" + ok,
		"no actions": head + "\nBUILD SUCCESSFUL in 2s\n",
		"no tasks":   "\nBUILD SUCCESSFUL in 2s\n1 actionable task: 1 executed\n",
		"task after": head + ok + "> Task :app:c\n",
	} {
		out, applied, reason := compressGradle([]byte(in))
		if applied || string(out) != in || !strings.HasPrefix(reason, "gradle_") {
			t.Errorf("%s: applied=%v reason=%q", name, applied, reason)
		}
	}
}

func TestDetectGradle(t *testing.T) {
	if f, _ := detectFormat([]byte("> Task :a\n\nBUILD SUCCESSFUL in 1s\n")); f != FormatGradle {
		t.Fatalf("format %q", f)
	}
	if f, fail := detectFormat([]byte("> Task :a FAILED\n\nBUILD FAILED in 1s\n")); f != FormatGradle || !fail {
		t.Fatalf("format %q fail=%v", f, fail)
	}
	if f, _ := detectFormat([]byte("BUILD SUCCESSFUL in 1s\n")); f == FormatGradle {
		t.Fatal("claimed output without a task line")
	}
}

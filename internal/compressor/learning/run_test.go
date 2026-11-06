// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package learning

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/execwrap"
)

// TestRun_RecordIsOnDiskWhenRunReturns runs an allowlisted command whose
// output the compressor does not recognise and checks the learning record is
// written before execwrap.Run returns (the hook used to run in a goroutine
// that process exit cut off).
func TestRun_RecordIsOnDiskWhenRunReturns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "learning.json")
	SetStorePath(path)
	defer SetStorePath("")

	bin := t.TempDir()
	script := "#!/bin/sh\nprintf 'zebra one /tmp/secret/path.txt\\nzebra two https://example.test/pr/1\\n'\n"
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	prev := execwrap.ObserveUncompactedHook
	execwrap.ObserveUncompactedHook = func(argv []string, sample []byte, family execwrap.Family) {
		_ = Record(argv, sample, family)
	}
	defer func() { execwrap.ObserveUncompactedHook = prev }()

	var out, errb bytes.Buffer
	r := execwrap.Run([]string{"go", "test", "./..."}, execwrap.Options{Stdin: strings.NewReader(""), Stdout: &out, Stderr: &errb, Compact: true})
	if r.ExitCode != 0 || r.Compacted {
		t.Fatalf("exit=%d compacted=%v stderr=%q", r.ExitCode, r.Compacted, errb.String())
	}
	if !strings.Contains(out.String(), "zebra one") {
		t.Fatalf("output not passed through: %q", out.String())
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("store not written when Run returned: %v", err)
	}
	for _, s := range []string{"zebra", "/tmp/secret", "path.txt", "example.test", "pr/1"} {
		if strings.Contains(string(data), s) {
			t.Errorf("store contains output text %q", s)
		}
	}
	if !strings.Contains(string(data), `"go test"`) && !strings.Contains(string(data), `"go",`) {
		t.Errorf("signature missing from store: %s", data)
	}
}

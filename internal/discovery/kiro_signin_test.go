// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// A kiro-cli that is not signed in must never run the list command: the list
// command is what opens the sign-in page. A marker file records any list call.
func TestKiroCLINotSignedInNeverRunsList(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "list-ran")
	bin := filepath.Join(dir, "kiro-cli")
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = whoami ]; then echo 'Not signed in'; exit 1; fi\n" +
		"touch '" + marker + "'\n" +
		"exit 0\n"
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	_, err := KiroCLI{Bin: bin}.Discover(context.Background())
	if err == nil {
		t.Fatal("expected an error when not signed in")
	}
	if _, statErr := os.Stat(marker); statErr == nil {
		t.Fatal("the list command ran without a sign-in")
	}
}

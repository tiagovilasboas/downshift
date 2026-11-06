// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"testing"
)

// useSessionList points the session list at a fixture written for the test,
// as the owner's operator file or a discovery cache would. The production code
// never ships a model list: a test that routes must supply the session list.
func useSessionList(t *testing.T, harness string, ids ...string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "session-models.json")
	body := `{"` + harness + `":[`
	for i, id := range ids {
		if i > 0 {
			body += ","
		}
		body += `"` + id + `"`
	}
	body += `]}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOWNSHIFT_SESSION_MODELS", path)
}

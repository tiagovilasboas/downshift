// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunRefusalNamesAllowlistAndTheDirectCommand(t *testing.T) {
	var stderr bytes.Buffer
	res := Run([]string{"rm", "-rf", "/tmp/x"}, Options{Stdout: &bytes.Buffer{}, Stderr: &stderr})
	if res.ExitCode != ExitCannotExecute || res.ExitClass != ExitClassRefused {
		t.Fatalf("exit %d class %q, want refused 126", res.ExitCode, res.ExitClass)
	}
	msg := strings.TrimSpace(stderr.String())
	if !strings.Contains(msg, "Allowed: "+AllowlistSummary()+"; ") {
		t.Errorf("refusal does not name the allowlist: %q", msg)
	}
	if !strings.HasSuffix(msg, "run the command without downshift compact") {
		t.Errorf("refusal must end with the direct-run hint: %q", msg)
	}
}

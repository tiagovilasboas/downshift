// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"strings"
	"testing"
)

// A single installed package reads "1 package", not "1 packages".
func TestPipInstallSingularPackageNoun(t *testing.T) {
	in := "Collecting requests==2.31.0\n  Downloading requests-2.31.0-py3-none-any.whl (62 kB)\n" +
		"Installing collected packages: requests\nSuccessfully installed requests-2.31.0\n"
	res := Compress([]byte(in), ModeSafe)
	if !res.Applied {
		t.Fatalf("pip install not compacted: reason=%s", res.Reason)
	}
	if got := string(res.Output); !strings.Contains(got, "[x] pip: installed 1 package") || strings.Contains(got, "1 packages") {
		t.Fatalf("output = %q, want the singular noun", got)
	}
}

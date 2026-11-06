// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

// compressGlab redacts the secrets in the output of a read-only glab command
// (FormatGlab: mr, issue, ci and pipeline listings and views). The text of
// those views is free-form, so nothing is cut: the only change is that secrets
// are redacted. An output with none stays as printed.
func compressGlab(input []byte) ([]byte, bool, string) {
	out, changed := RedactSecrets(string(input))
	if !changed {
		return input, false, "no_compression_gain"
	}
	return []byte(out), true, "glab secrets redacted"
}

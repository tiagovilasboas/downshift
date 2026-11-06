// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package ccr

import (
	"bytes"
	"regexp"
)

// reSecret flags a line that looks like it carries a credential: the words of
// reGitDiffSecret in internal/compressor (ccr must not import it), plus auth
// headers, cookies, credentials in a URL, JWTs, well-known key prefixes and
// env-style PASS/AUTH assignments.
//
// The generic words (secret, token, cookie, authorization) count only as the
// name of an assignment or header, `name = value` or `name: value`. Bare, they
// withheld ordinary tool output: Downshift's own test names and savings lines
// say "token" all the time, and a path like internal/ccr/secret.go is not a
// credential.
var reSecret = regexp.MustCompile(`(?i)(\bsecrets?\s*[=:]|_secret\s*[=:]|\btokens?\s*[=:]|_token\s*[=:]|password|passwd|api[_-]?key|private[_-]?key|access[_-]?key|aws_secret|bearer |\bauthorization\s*:|cookie\s*[=:]|PRIVATE KEY|://[^/\s:@]+:[^@\s]+@|\beyJ[\w-]{10,}\.|\bgh[pousr]_[A-Za-z0-9]|\bsk-[A-Za-z0-9]|\bAKIA[0-9A-Z]{16}|[_.-]pass(wd|word)?\s*[=:]|\bpass(wd|word)?\s*=|_auth\s*[=:])`)

// rePEMBegin and rePEMEnd bound a PEM block; every line between them, the
// base64 body included, is withheld.
var (
	rePEMBegin = regexp.MustCompile(`-----BEGIN [A-Z0-9 ]+-----`)
	rePEMEnd   = regexp.MustCompile(`-----END [A-Z0-9 ]+-----`)
)

const redactedLine = "[downshift ccr: line withheld, it looked like a secret]"

// redact withholds every line that looks like a secret and every line of a PEM
// block, from its BEGIN line through its END line. An unterminated block
// withholds the rest of the output.
func redact(raw []byte) []byte {
	if !reSecret.Match(raw) && !rePEMBegin.Match(raw) {
		return raw
	}
	lines := bytes.Split(raw, []byte("\n"))
	inPEM := false
	for i, l := range lines {
		if !inPEM && rePEMBegin.Match(l) {
			inPEM = true
		}
		if inPEM || reSecret.Match(l) {
			if inPEM && rePEMEnd.Match(l) {
				inPEM = false
			}
			lines[i] = []byte(redactedLine)
		}
	}
	return bytes.Join(lines, []byte("\n"))
}

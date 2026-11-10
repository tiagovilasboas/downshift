// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import "github.com/tiagovilasboas/downshift-ai/internal/lazyre"

// redactedMark replaces a secret in a compacted view.
const redactedMark = "[redacted]"

var (
	// A GitLab token: personal access, deploy, pipeline trigger, runner and
	// the other glXXX- prefixed kinds, followed by 20 or more token characters.
	reGitLabToken = lazyre.New(`\b(?:glpat|gldt|glptt|glrt|glcbt|glimt|glagent|glft|glsoat|gloas)-[A-Za-z0-9_\-]{20,}`)
	// A PRIVATE-TOKEN, JOB-TOKEN or DEPLOY-TOKEN header or assignment.
	reTokenHeader = lazyre.New(`(?i)\b((?:private|job|deploy)-token\s*[:=]\s*)[^\s"']+`)
	// An Authorization bearer value.
	reBearer = lazyre.New(`(?i)\b(bearer\s+)[A-Za-z0-9._~+/=\-]{8,}`)
	// user:token@ (or token@) in front of the host of a URL.
	reURLCredentials = lazyre.New(`(?i)\b([a-z][a-z0-9+.\-]*://)[^/\s@"']+@`)
)

// RedactSecrets replaces GitLab tokens, PRIVATE-TOKEN and Bearer values and
// the credentials of a URL by [redacted]. It reports whether anything changed.
// It is shared by every compressor whose command can print such a value.
func RedactSecrets(text string) (string, bool) {
	out := reURLCredentials.ReplaceAllString(text, "${1}"+redactedMark+"@")
	out = reGitLabToken.ReplaceAllString(out, redactedMark)
	out = reTokenHeader.ReplaceAllString(out, "${1}"+redactedMark)
	out = reBearer.ReplaceAllString(out, "${1}"+redactedMark)
	return out, out != text
}

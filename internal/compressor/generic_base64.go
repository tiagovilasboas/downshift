// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/lazyre"
)

// minBase64Run is the shortest run of base64 characters that is elided.
const minBase64Run = 1024

var reDataURI = lazyre.New(`data:([A-Za-z0-9.+-]+/[A-Za-z0-9.+-]+);base64,$`)

func isBase64Byte(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '+' || c == '/'
}

// looksBase64 reports whether a run mixes upper case, lower case and digits,
// as encoded binary does. A run of one character class (a rule of '+', a
// hex digest, a long lower-case word list) is not a blob.
func looksBase64(run string) bool {
	var up, low, dig bool
	for i := 0; i < len(run); i++ {
		switch c := run[i]; {
		case c >= 'A' && c <= 'Z':
			up = true
		case c >= 'a' && c <= 'z':
			low = true
		case c >= '0' && c <= '9':
			dig = true
		}
	}
	return up && low && dig
}

// elideBase64 replaces every run of at least 1 KB of base64 characters with a
// one-line marker such as [base64 image/png, 50 KB elided] and returns how
// many it replaced. It scans once, byte by byte, so a 2 MB line costs a
// single pass.
func elideBase64(text string) (string, int) {
	n := 0
	var b strings.Builder
	last := 0
	for i := 0; i < len(text); {
		if !isBase64Byte(text[i]) {
			i++
			continue
		}
		j := i
		for j < len(text) && isBase64Byte(text[j]) {
			j++
		}
		end := j
		for end < len(text) && end-j < 2 && text[end] == '=' {
			end++
		}
		if run := text[i:j]; len(run) >= minBase64Run && looksBase64(run) {
			pre := text[last:i]
			label := "base64"
			if k := strings.LastIndex(pre, "data:"); k >= 0 {
				if m := reDataURI.FindStringSubmatchIndex(pre[k:]); m != nil {
					label = "base64 " + pre[k+m[2]:k+m[3]]
					pre = pre[:k]
				}
			}
			b.WriteString(pre)
			fmt.Fprintf(&b, "[%s, %s elided]", label, humanKB(len(run)*3/4))
			last = end
			n++
		}
		i = end
	}
	b.WriteString(text[last:])
	return b.String(), n
}

// humanKB renders a byte count as whole KB, at least 1.
func humanKB(n int) string {
	return fmt.Sprintf("%d KB", max(1, (n+512)/1024))
}

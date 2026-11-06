// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// The capped view of an HTTP fetch (curl, wget -O -).
const (
	// HTTPFetchMinBytes is the shortest response that is capped.
	HTTPFetchMinBytes = 2048
	fetchBodyHead     = 1024 // first body bytes kept
	fetchBodyTail     = 512  // last body bytes kept
	fetchMinCut       = 256  // a body cut by less than this is kept whole
	fetchMaxDanger    = 30   // danger lines kept from the cut stretch; more stays raw
	fetchMaxRatio     = 70   // the view must be under this percent of the input
)

// fetchKeyHeaders are the response headers worth the model's attention; every
// other header is counted and dropped.
var fetchKeyHeaders = map[string]bool{
	"content-type": true, "content-length": true, "content-encoding": true,
	"location": true, "cache-control": true, "etag": true, "last-modified": true,
	"retry-after": true, "server": true,
}

// fetchSecretWords in a header name mark a value that is never shown.
var fetchSecretWords = []string{"cookie", "auth", "token", "secret", "session", "credential", "key"}

// compressHTTPFetch caps a long response: for every status line of the
// transfer (curl -i, -I) the status, and for the last block its key headers,
// then the first and last bytes of the body with a count of what was cut. The
// value of a header whose name looks like a credential (Set-Cookie,
// Authorization, ...) is replaced by [redacted]. A response that is short, a
// status of 400 or more, or a view that is not shorter stays raw.
func compressHTTPFetch(input []byte, h CommandHint) ([]byte, bool, string) {
	if len(input) < HTTPFetchMinBytes {
		return input, false, "no_compression_gain"
	}
	text := string(input)
	var b strings.Builder
	if h.HTTPHeaders {
		var ok bool
		if text, ok = writeFetchHeaders(&b, text); !ok {
			return input, false, "http_status_preserved"
		}
	}
	cut := writeFetchBody(&b, text)
	if cut < 0 {
		return input, false, "http_danger_lines_preserved"
	}
	if b.Len() > 0 && !strings.HasSuffix(b.String(), "\n") {
		b.WriteString("\n")
	}
	if cut > 0 {
		fmt.Fprintf(&b, "[downshift fetch: %d body bytes cut; run the command again for the whole response]\n", cut)
	}
	if b.Len()*100 >= len(input)*fetchMaxRatio {
		return input, false, "no_compression_gain"
	}
	return []byte(b.String()), true, "HTTP response capped"
}

// writeFetchHeaders writes the header blocks at the start of text and returns
// the body that follows. It reports false when the text does not start with a
// status line or the last status is 400 or more.
func writeFetchHeaders(b *strings.Builder, text string) (string, bool) {
	if !strings.HasPrefix(text, "HTTP/") {
		return text, false
	}
	for strings.HasPrefix(text, "HTTP/") {
		block, rest := splitHeaderBlock(text)
		lines := strings.Split(strings.ReplaceAll(block, "\r", ""), "\n")
		code := 0
		if f := strings.Fields(lines[0]); len(f) >= 2 {
			code, _ = strconv.Atoi(f[1])
		}
		if code >= 400 || code == 0 {
			return text, false
		}
		b.WriteString(lines[0] + "\n")
		writeKeyHeaders(b, lines[1:], !strings.HasPrefix(rest, "HTTP/"))
		text = rest
	}
	b.WriteString("\n")
	return text, true
}

// splitHeaderBlock splits text at its first blank line.
func splitHeaderBlock(text string) (block, rest string) {
	for _, sep := range []string{"\r\n\r\n", "\n\n"} {
		if i := strings.Index(text, sep); i >= 0 {
			return text[:i], text[i+len(sep):]
		}
	}
	return text, ""
}

// writeKeyHeaders writes the key headers of one block (all of them for the
// last block, only Location for a redirect hop) and counts the rest.
func writeKeyHeaders(b *strings.Builder, lines []string, last bool) {
	dropped := 0
	for _, l := range lines {
		name, _, found := strings.Cut(l, ":")
		lower := strings.ToLower(strings.TrimSpace(name))
		switch {
		case !found || lower == "":
		case fetchSecretName(lower):
			fmt.Fprintf(b, "%s: [redacted]\n", strings.TrimSpace(name))
			continue
		case fetchKeyHeaders[lower] && (last || lower == "location"), found && reDangerWord.MatchString(l):
			b.WriteString(strings.TrimSpace(l) + "\n")
			continue
		}
		dropped++
	}
	if dropped > 0 {
		fmt.Fprintf(b, "[... %d other headers]\n", dropped)
	}
}

func fetchSecretName(lower string) bool {
	for _, w := range fetchSecretWords {
		if strings.Contains(lower, w) {
			return true
		}
	}
	return false
}

// writeFetchBody writes the head and tail of body, and between them every line
// of the cut stretch that holds a danger word, whole. It returns the bytes cut.
// A stretch with more than fetchMaxDanger such lines is not cut: it returns -1
// and writes nothing.
func writeFetchBody(b *strings.Builder, body string) int {
	if len(body) < fetchBodyHead+fetchBodyTail+fetchMinCut {
		b.WriteString(body)
		return 0
	}
	head := fetchBodyHead
	for head > 0 && !utf8.RuneStart(body[head]) {
		head--
	}
	tail := len(body) - fetchBodyTail
	for tail < len(body) && !utf8.RuneStart(body[tail]) {
		tail++
	}
	var kept []string
	for _, l := range strings.Split(body[head:tail], "\n") {
		if reDangerWord.MatchString(l) {
			kept = append(kept, l)
		}
	}
	if len(kept) > fetchMaxDanger {
		return -1
	}
	cut := tail - head
	b.WriteString(body[:head] + "\n[... " + strconv.Itoa(cut) + " bytes cut ...]\n")
	for _, l := range kept {
		b.WriteString(l + "\n")
		cut -= len(l) + 1
	}
	b.WriteString(body[tail:])
	return max(cut, 1)
}

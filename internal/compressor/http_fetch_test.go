// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"strings"
	"testing"
)

const fetchSecretValue = "abc123def456SECRET"

func fetchResponse(status string, body string) string {
	return "HTTP/2 " + status + "\r\n" +
		"content-type: text/html\r\n" +
		"set-cookie: session=" + fetchSecretValue + "; HttpOnly\r\n" +
		"Authorization: Bearer " + fetchSecretValue + "\r\n" +
		"x-api-key: " + fetchSecretValue + "\r\n" +
		"x-trace-id: 77\r\nvary: accept\r\n\r\n" + body
}

func TestHTTPFetchCapsHeadersAndBody(t *testing.T) {
	body := strings.Repeat("<p>line of a long page</p>\n", 300)
	in := fetchResponse("200", body)
	out, ok, _ := compressHTTPFetch([]byte(in), CommandHint{HTTPHeaders: true})
	if !ok {
		t.Fatal("not compacted")
	}
	text := string(out)
	for _, want := range []string{
		"HTTP/2 200\n", "content-type: text/html\n", "set-cookie: [redacted]\n",
		"Authorization: [redacted]\n", "x-api-key: [redacted]\n", "[... 2 other headers]\n",
		"bytes cut ...]\n", "body bytes cut; run the command again",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("view lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, fetchSecretValue) || strings.Contains(text, "Bearer") || strings.Contains(text, "\r") {
		t.Errorf("view shows a credential or a CR:\n%s", text)
	}
	if len(out) > 2200 {
		t.Errorf("view is %d bytes", len(out))
	}
}

func TestHTTPFetchBodyOnlyAndRedirects(t *testing.T) {
	body := strings.Repeat("héllo wörld ünïcode\n", 200)
	out, ok, _ := compressHTTPFetch([]byte(body), CommandHint{})
	if !ok || !strings.Contains(string(out), "bytes cut ...]") || !validUTF8(out) {
		t.Fatalf("body only: ok=%v valid=%v", ok, validUTF8(out))
	}
	hop := "HTTP/1.1 302 Found\r\nlocation: https://example.com/next\r\nset-cookie: s=" + fetchSecretValue + "\r\n\r\n"
	in := hop + fetchResponse("200", strings.Repeat("x", 4000)+"\n")
	out, ok, _ = compressHTTPFetch([]byte(in), CommandHint{HTTPHeaders: true})
	text := string(out)
	if !ok || !strings.Contains(text, "HTTP/1.1 302 Found\nlocation: https://example.com/next\nset-cookie: [redacted]\nHTTP/2 200\n") || strings.Contains(text, fetchSecretValue) {
		t.Errorf("redirect chain: ok=%v\n%s", ok, text)
	}
}

func validUTF8(b []byte) bool { return strings.ToValidUTF8(string(b), "") == string(b) }

func TestHTTPFetchStaysRaw(t *testing.T) {
	long := strings.Repeat("a line of the body\n", 300)
	for _, tc := range []struct {
		name, in string
		hint     CommandHint
		cause    string
	}{
		{"short", fetchResponse("200", "ok\n"), CommandHint{HTTPHeaders: true}, "no_compression_gain"},
		{"client error", fetchResponse("404 Not Found", long), CommandHint{HTTPHeaders: true}, "http_status_preserved"},
		{"server error", fetchResponse("503", long), CommandHint{HTTPHeaders: true}, "http_status_preserved"},
		{"no status line", long, CommandHint{HTTPHeaders: true}, "http_status_preserved"},
		{"unreadable status", "HTTP/2 abc\r\n\r\n" + long, CommandHint{HTTPHeaders: true}, "http_status_preserved"},
	} {
		out, ok, reason := compressHTTPFetch([]byte(tc.in), tc.hint)
		if ok || string(out) != tc.in || reason != tc.cause {
			t.Errorf("%s: ok=%v reason=%q (want raw, %q)", tc.name, ok, reason, tc.cause)
		}
	}
	res := CompressCommandOutputHint([]byte("\x1b[2Jscreen\n"+long), FormatHTTPFetch, 0, CommandHint{})
	if res.Applied {
		t.Error("an escape sequence was compacted")
	}
	if res = CompressCommandOutputHint([]byte(fetchResponse("200", long)), FormatHTTPFetch, 22, CommandHint{HTTPHeaders: true}); res.Applied {
		t.Error("a failed transfer was compacted")
	}
}

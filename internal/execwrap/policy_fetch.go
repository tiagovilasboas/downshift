// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"strings"

	"github.com/tiagovilasboas/downshift/internal/compressor"
)

// ════════════════════════════════════════════════════════════════════════════
// DOMAIN: reading a URL — curl and wget -O - (read-only fetches)
// ════════════════════════════════════════════════════════════════════════════

// Fetch families. Only a fetch that prints the response and sends nothing but
// a plain GET or HEAD is a family. The accepted words are listed, not the
// refused ones: every other option (-o, -O, -d, --data*, -T, -F, -K, -H, -u,
// -b, -x, --proxy, a method but GET or HEAD, ...) leaves the command as written.
const (
	FamilyCurl Family = "curl"
	FamilyWget Family = "wget"
)

// rawStoreRefused lists the families whose raw output is never offered to the
// raw-output store (Options.Keep): a response can carry Set-Cookie and other
// credential headers, and glab output may carry a token in a URL or a header;
// the compacted view redacts both.
var rawStoreRefused = map[Family]bool{FamilyCurl: true, FamilyWget: true, FamilyGlab: true}

// curlShortFlags are the letters a cluster such as -sSL may hold: silent,
// show-error, include headers, head, fail, follow redirects.
const curlShortFlags = "sSiIfL"

// curlLongFlags take no value.
var curlLongFlags = map[string]bool{
	"--silent": true, "--show-error": true, "--include": true, "--head": true,
	"--fail": true, "--location": true, "--compressed": true,
}

// curlNumberFlags take a whole number of seconds.
var curlNumberFlags = map[string]bool{"-m": true, "--max-time": true, "--connect-timeout": true}

// fetchFamily names the curl or wget family of argv, or FamilyNone.
//
//	curl [-sSiIfL cluster] [--silent ...] [-m N] [-X GET|HEAD] URL
//	wget [-q] (-O - | -O- | -qO- | --output-document=-) URL
func fetchFamily(argv []string) Family {
	switch argv[0] {
	case "curl":
		if curlReadOnly(argv[1:]) {
			return FamilyCurl
		}
	case "wget":
		if wgetToStdout(argv[1:]) {
			return FamilyWget
		}
	}
	return FamilyNone
}

// curlReadOnly accepts the flags above and exactly one http(s) URL.
func curlReadOnly(args []string) bool {
	urls := 0
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case curlLongFlags[a]:
		case a == "-X" || a == "--request":
			i++
			if i >= len(args) || (args[i] != "GET" && args[i] != "HEAD") {
				return false
			}
		case curlNumberFlags[a]:
			i++
			if i >= len(args) || !smallNumber(args[i]) {
				return false
			}
		case strings.HasPrefix(a, "-") && len(a) > 1 && !strings.HasPrefix(a, "--"):
			if !onlyFlags(a[1:], curlShortFlags) {
				return false
			}
		case strings.HasPrefix(a, "-"):
			return false
		default:
			urls++
			if !plainHTTPURL(a) {
				return false
			}
		}
	}
	return urls == 1
}

// wgetToStdout accepts `-O -` in any of its spellings, optionally quiet, and
// exactly one http(s) URL. wget without it writes a file.
func wgetToStdout(args []string) bool {
	toStdout, urls := false, 0
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "-q" || a == "--quiet":
		case a == "-O-" || a == "-qO-" || a == "--output-document=-":
			toStdout = true
		case a == "-O" || a == "--output-document":
			i++
			if i >= len(args) || args[i] != "-" {
				return false
			}
			toStdout = true
		case strings.HasPrefix(a, "-"):
			return false
		default:
			urls++
			if !plainHTTPURL(a) {
				return false
			}
		}
	}
	return toStdout && urls == 1
}

// plainHTTPURL accepts an http or https URL with a host, no credentials in the
// authority (user:pass@host), no glob or control characters and a bounded length.
func plainHTTPURL(u string) bool {
	rest, ok := strings.CutPrefix(u, "https://")
	if !ok {
		rest, ok = strings.CutPrefix(u, "http://")
	}
	if !ok || rest == "" || len(u) > 400 || strings.ContainsAny(u, "[]{}\\ \t\n\r") {
		return false
	}
	for i := 0; i < len(u); i++ {
		if u[i] < 0x20 || u[i] == 0x7f {
			return false
		}
	}
	authority := rest
	if i := strings.IndexAny(rest, "/?#"); i >= 0 {
		authority = rest[:i]
	}
	return authority != "" && !strings.Contains(authority, "@")
}

// smallNumber is a whole number of at most four digits.
func smallNumber(s string) bool {
	return s != "" && len(s) <= 4 && strings.Trim(s, "0123456789") == ""
}

// fetchHint tells the compressor whether the response headers are printed.
func fetchHint(argv []string) compressor.CommandHint {
	h := compressor.CommandHint{}
	if argv[0] != "curl" {
		return h
	}
	for _, a := range argv[1:] {
		short := strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && strings.ContainsAny(a, "iI")
		if short || a == "--include" || a == "--head" {
			h.HTTPHeaders = true
		}
	}
	return h
}

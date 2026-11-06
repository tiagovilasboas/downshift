// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"strings"
	"testing"
)

func TestFamilyOfFetch(t *testing.T) {
	for _, tc := range []struct {
		command string
		want    Family
	}{
		{"curl https://example.com/api", FamilyCurl},
		{"curl -s https://example.com/api", FamilyCurl},
		{"curl -sSL https://example.com/api", FamilyCurl},
		{"curl -i http://localhost:8080/health", FamilyCurl},
		{"curl -I https://example.com", FamilyCurl},
		{"curl --silent --include --location https://example.com", FamilyCurl},
		{"curl -X GET https://example.com", FamilyCurl},
		{"curl -X HEAD https://example.com", FamilyCurl},
		{"curl --request GET https://example.com", FamilyCurl},
		{"curl -m 10 https://example.com", FamilyCurl},
		{"curl --max-time 10 --connect-timeout 5 -fsS https://example.com/x?a=1&b=2", FamilyCurl},
		{"wget -qO- https://example.com", FamilyWget},
		{"wget -O- https://example.com", FamilyWget},
		{"wget -q -O - https://example.com", FamilyWget},
		{"wget --output-document=- https://example.com", FamilyWget},
		{"wget --output-document - https://example.com", FamilyWget},
	} {
		if got := FamilyOf(strings.Fields(tc.command)); got != tc.want {
			t.Errorf("FamilyOf(%q) = %q, want %q", tc.command, got, tc.want)
		}
	}
}

// Every form that writes a file, sends data, carries credentials or is not a
// plain GET or HEAD of one web address is left as written.
func TestFamilyOfFetchRefusals(t *testing.T) {
	for _, command := range []string{
		// curl: output to a file
		"curl -o out.html https://example.com", "curl -O https://example.com/a.tgz",
		"curl --output out.html https://example.com", "curl --output-dir /tmp https://example.com",
		"curl -sSo out https://example.com", "curl -J -O https://example.com", "curl --remote-name https://example.com",
		"curl -c jar.txt https://example.com", "curl --cookie-jar jar.txt https://example.com",
		"curl --create-dirs https://example.com", "curl --trace trace.txt https://example.com",
		"curl -D headers.txt https://example.com", "curl --dump-header h.txt https://example.com",
		// curl: sends data or a file
		"curl -d a=1 https://example.com", "curl --data a=1 https://example.com", "curl --data-raw a https://example.com",
		"curl --data-binary @f https://example.com", "curl --data-urlencode a=b https://example.com",
		"curl -T file https://example.com", "curl --upload-file file https://example.com",
		"curl -F a=b https://example.com", "curl --form a=b https://example.com", "curl --json x https://example.com",
		"curl -G -d a=b https://example.com",
		// curl: methods
		"curl -X POST https://example.com", "curl -X PUT https://example.com", "curl -X DELETE https://example.com",
		"curl -X PATCH https://example.com", "curl -X get https://example.com", "curl -X", "curl -X GET",
		"curl --request=POST https://example.com", "curl -XPOST https://example.com",
		// curl: config, credentials, proxies, certificates
		"curl -K cfg https://example.com", "curl --config cfg https://example.com",
		"curl -H Authorization:x https://example.com", "curl --header X:y https://example.com",
		"curl -u user:pass https://example.com", "curl --user user:pass https://example.com",
		"curl -b a=b https://example.com", "curl --cookie a=b https://example.com",
		"curl -x proxy:1 https://example.com", "curl --proxy proxy:1 https://example.com",
		"curl --oauth2-bearer tok https://example.com", "curl -n https://example.com", "curl --netrc https://example.com",
		"curl -E cert https://example.com", "curl --cert cert https://example.com", "curl --key k https://example.com",
		"curl -e ref https://example.com", "curl -A agent https://example.com", "curl -k https://example.com",
		"curl --insecure https://example.com", "curl -v https://example.com", "curl -w x https://example.com",
		// curl: addresses
		"curl", "curl -s", "curl https://example.com https://example.org", "curl example.com",
		"curl file:///etc/passwd", "curl ftp://example.com/a", "curl scp://example.com/a", "curl gopher://x/",
		"curl https://user:pw@example.com", "curl https://user@example.com", "curl https://", "curl http:///path",
		"curl https://example.com/[1-5]", "curl https://example.com/{a,b}", "curl -- https://example.com",
		"curl -m abc https://example.com", "curl -m 99999 https://example.com", "curl -m", "curl -",
		"curl --silent=1 https://example.com", "curl -L -- https://example.com",
		// wget: writes a file unless the document goes to standard output
		"wget https://example.com", "wget -q https://example.com", "wget -O out.html https://example.com",
		"wget -O out https://example.com", "wget -qO out https://example.com", "wget --output-document=out https://example.com",
		"wget -o log https://example.com", "wget --output-file=log https://example.com", "wget -P dir https://example.com",
		"wget -O - -O out https://example.com", "wget -O", "wget -O -", "wget -qO-",
		"wget -r -O- https://example.com", "wget -i list -O- https://example.com", "wget --post-data=a=1 -O- https://example.com",
		"wget --post-file=f -O- https://example.com", "wget --header=X:y -O- https://example.com",
		"wget --user=u -O- https://example.com", "wget --load-cookies=c -O- https://example.com",
		"wget -O- https://example.com https://example.org", "wget -O- ftp://example.com/a", "wget -O- file:///etc/passwd",
		"wget -O- https://u:p@example.com",
	} {
		if got := FamilyOf(strings.Fields(command)); got != FamilyNone {
			t.Errorf("FamilyOf(%q) = %q, want none", command, got)
		}
	}
}

func TestDecideFetchWrapsOnlyThePlainForm(t *testing.T) {
	const exe = "/usr/local/bin/downshift"
	if d := Decide("curl -sS https://example.com/api", exe); !d.Rewrite || d.Family != FamilyCurl {
		t.Errorf("plain curl not wrapped: %+v", d)
	}
	for _, command := range []string{
		"curl -s https://example.com | jq .", "curl -s https://example.com > out", "curl -d @f https://example.com",
		"curl -H 'Authorization: Bearer x' https://example.com", "curl -s https://example.com -o out",
		"curl -u 'a:b' https://example.com", "curl -X 'POST' https://example.com",
	} {
		if d := Decide(command, exe); d.Rewrite && (d.Family == FamilyCurl || d.Family == FamilyWget) {
			t.Errorf("Decide(%q) wrapped a non-read fetch: %+v", command, d)
		}
	}
}

// env and printenv print secret values, and a name-based redaction cannot see
// a password inside a URL value, so they are not a family. They run as written.
func TestEnvAndPrintenvAreNotFamilies(t *testing.T) {
	for _, command := range []string{"env", "printenv", "printenv HOME", "env FOO=1", "env -i"} {
		if got := FamilyOf(strings.Fields(command)); got != FamilyNone {
			t.Errorf("FamilyOf(%q) = %q, want none", command, got)
		}
	}
}

func TestFetchHint(t *testing.T) {
	for _, tc := range []struct {
		command string
		want    bool
	}{
		{"curl -i https://example.com", true}, {"curl -sI https://example.com", true},
		{"curl --include https://example.com", true}, {"curl --head https://example.com", true},
		{"curl -s https://example.com", false}, {"wget -qO- https://example.com", false},
		{"curl -sS https://example.com/-i", false},
	} {
		if got := fetchHint(strings.Fields(tc.command)).HTTPHeaders; got != tc.want {
			t.Errorf("fetchHint(%q).HTTPHeaders = %v, want %v", tc.command, got, tc.want)
		}
	}
}

// Credentials never reach the raw-output store.
func TestFetchFamiliesRefuseRawStore(t *testing.T) {
	for _, f := range []Family{FamilyCurl, FamilyWget} {
		if !rawStoreRefused[f] {
			t.Errorf("%s may store raw output", f)
		}
	}
}

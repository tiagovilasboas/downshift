// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"slices"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// Web and host security vocabulary added to the risk table: CORS, cookie
// attributes, traversal, permissive modes, SSH and API tokens. Each phrase is
// a surface a rename is not mechanical on.
func TestRiskVocabulary_Positive(t *testing.T) {
	for p, want := range map[string]string{
		"relax the CORS policy to allow all origins":     "vuln",
		"rename the cors middleware":                     "vuln",
		"set SameSite=Lax on the cookie":                 "vuln",
		"switch the session cookie to HttpOnly":          "vuln",
		"rename the authCookie option":                   "vuln",
		"fix path traversal in the download handler":     "vuln",
		"block directory traversal in the static server": "vuln",
		"patch the zip slip in the extractor":            "vuln",
		"close the open redirect on /login":              "vuln",
		"add a clickjacking header":                      "vuln",
		"tighten the Content-Security-Policy":            "vuln",
		"the container runs as root, rename the user":    "vuln",
		"the public bucket serves the uploads":           "vuln",
		"corrija a travessia de diretório no upload":     "vuln",
		"renomeie o cookie de sessão":                    "vuln",
		"chmod 777 the uploads directory":                "destructive",
		"run chmod -R 777 on the cache":                  "destructive",
		"chmod a+rwx the script":                         "destructive",
		"the directory is world-writable":                "destructive",
		"add the key to the ssh key list":                "secret",
		"rename sshKey to deployKey":                     "secret",
		"append it to authorized_keys":                   "secret",
		"rotate the API token":                           "secret",
		"rename apiToken to serviceToken":                "secret",
		"store the personal access token in the config":  "secret",
	} {
		if cats := core.RiskCategories(p); !slices.Contains(cats, want) {
			t.Errorf("RiskCategories(%q) = %v, want %s", p, cats, want)
		}
	}
}

// Near misses that must not raise the floor.
func TestRiskVocabulary_NoFalsePositives(t *testing.T) {
	for _, p := range []string{
		"rename the Corsair component to Engine",
		"update the Scorsese movie list",
		"fix the traversal of the AST in walk.go",
		"rename the tree traversal helper to visit",
		"update the cookie banner text",
		"rename the cookie jar variable to jar",
		"chmod 644 the config file",
		"run the build as rootless",
		"rename the root directory constant",
		"find the root cause of the flaky test",
		"world map tiles are cached",
		"rename the bucket sort helper",
		"the public API docs mention buckets of requests",
		"add a redirect from /old to /new in the docs site",
		"rename the personal data table name",
		"rename the token counter in the tokenizer",
		"rename the policy file",
		"renomeie o banner de cookies",
		"renomeie a travessia da árvore para visitar",
	} {
		for _, c := range core.RiskCategories(p) {
			if c == "vuln" || c == "destructive" || c == "secret" {
				t.Errorf("%q matched %v", p, core.RiskCategories(p))
			}
		}
	}
}

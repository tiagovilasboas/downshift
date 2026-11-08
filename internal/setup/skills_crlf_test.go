// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/setup/configfile"
)

// The ownership note goes after the frontmatter whatever the line endings, so
// a skill built from a CRLF checkout still opens with its frontmatter.
func TestOwnedMarkdownKeepsFrontmatterFirst(t *testing.T) {
	for _, nl := range []string{"\n", "\r\n"} {
		body := "---" + nl + "name: x" + nl + "---" + nl + "text" + nl
		got := ownedMarkdown(body)
		if !strings.HasPrefix(got, "---"+nl+"name: x"+nl+"---"+nl+configfile.OwnedNote) {
			t.Errorf("%q: note not after the frontmatter: %q", nl, got)
		}
	}
	if got := ownedMarkdown("plain"); got != configfile.OwnedNote+"plain" {
		t.Errorf("no frontmatter: %q", got)
	}
}

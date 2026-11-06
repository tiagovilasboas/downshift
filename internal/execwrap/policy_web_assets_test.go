// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"strings"
	"testing"
)

func TestFamilyOf_WebAssets(t *testing.T) {
	cases := []struct {
		command string
		want    Family
	}{
		{"stylelint src/**/*.css", FamilyStylelint},
		{"npx stylelint src/app.css --formatter verbose", FamilyStylelint},
		{"htmlhint index.html", FamilyHTMLHint},
		{"npx htmlhint dist/", FamilyHTMLHint},
		{"html-validate dist/*.html", FamilyHTMLValidate},
		{"npx html-validate index.html", FamilyHTMLValidate},
		{"tailwindcss -i in.css -o out.css", FamilyTailwind},
		{"tailwindcss build -i in.css -o out.css --minify", FamilyTailwind},
		{"npx tailwindcss -i in.css -o out.css", FamilyTailwind},
		{"npx @tailwindcss/cli -i in.css -o out.css", FamilyTailwind},
		{"sass in.scss out.css", FamilySass},
		{"npx sass --style=compressed in.scss out.css", FamilySass},
		{"postcss in.css -o out.css", FamilyPostCSS},
		{"npx postcss in.css -o out.css", FamilyPostCSS},
		// refused: acting, interactive, never exiting or remote
		{"stylelint --fix src/app.css", FamilyNone},
		{"npx stylelint src/app.css --fix", FamilyNone},
		{"stylelint https://example.com/a.css", FamilyNone},
		{"html-validate --fix index.html", FamilyNone},
		{"htmlhint http://example.com/index.html", FamilyNone},
		{"tailwindcss init", FamilyNone},
		{"npx tailwindcss init -p", FamilyNone},
		{"tailwindcss -i in.css -o out.css --watch", FamilyNone},
		{"tailwindcss -i in.css -o out.css -w", FamilyNone},
		{"npx @tailwindcss/cli -i in.css -o out.css --watch=always", FamilyNone},
		{"sass --watch in.scss out.css", FamilyNone},
		{"sass -i", FamilyNone},
		{"sass --stdin", FamilyNone},
		{"sass", FamilyNone},
		{"sass https://example.com/a.scss out.css", FamilyNone},
		{"postcss in.css -o out.css --watch", FamilyNone},
		{"postcss in.css -o out.css -w", FamilyNone},
		{"postcss in.css --replace", FamilyNone},
		{"postcss in.css -r", FamilyNone},
		{"postcss", FamilyNone},
		{"npx", FamilyNone},
		{"npx cowsay hi", FamilyNone},
	}
	for _, c := range cases {
		if got := FamilyOf(strings.Fields(c.command)); got != c.want {
			t.Errorf("FamilyOf(%q) = %q, want %q", c.command, got, c.want)
		}
	}
}

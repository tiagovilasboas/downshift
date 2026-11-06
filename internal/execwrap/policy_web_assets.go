// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "strings"

// DOMAIN: Frontend web assets, CSS, HTML and Tailwind tooling.

// Web-asset family constants.
const (
	FamilyStylelint    Family = "stylelint"
	FamilyHTMLHint     Family = "htmlhint"
	FamilyHTMLValidate Family = "html-validate"
	FamilyTailwind     Family = "tailwindcss"
	FamilySass         Family = "sass"
	FamilyPostCSS      Family = "postcss"
)

// webAssetFamily detects stylelint, htmlhint, html-validate, tailwindcss
// (also `npx @tailwindcss/cli`), sass and postcss, bare or through `npx`.
// Their output is a log, so a clean run goes through the generic chain only.
//
// Refused: `--watch` and `-w` (never exit), `--fix` (rewrites files),
// tailwind `init` and htmlhint/html-validate `--init` (write a config), postcss `--replace`/`-r` (overwrites its
// input), sass `-i`/`--interactive`/`--stdin` and a bare `sass`/`postcss`
// with no argument (read stdin), and any argument naming a remote resource
// (`://`).
func webAssetFamily(argv []string) Family {
	if len(argv) == 0 {
		return FamilyNone
	}
	prog, args := argv[0], argv[1:]
	if prog == "npx" {
		if len(argv) < 2 {
			return FamilyNone
		}
		prog, args = argv[1], argv[2:]
	}
	var family Family
	switch prog {
	case "stylelint":
		family = FamilyStylelint
	case "htmlhint":
		family = FamilyHTMLHint
	case "html-validate":
		family = FamilyHTMLValidate
	case "tailwindcss", "@tailwindcss/cli":
		family = FamilyTailwind
	case "sass":
		family = FamilySass
	case "postcss":
		family = FamilyPostCSS
	default:
		return FamilyNone
	}
	if webAssetRefused(family, args) {
		return FamilyNone
	}
	return family
}

// webAssetRefused reports whether args hold an acting, interactive,
// never-exiting or remote form for the family.
func webAssetRefused(family Family, args []string) bool {
	if (family == FamilySass || family == FamilyPostCSS) && len(args) == 0 {
		return true
	}
	for _, a := range args {
		if strings.Contains(a, "://") || strings.HasPrefix(a, "--fix") {
			return true
		}
		switch family {
		case FamilyHTMLHint, FamilyHTMLValidate:
			if a == "--init" {
				return true
			}
		case FamilyTailwind:
			if a == "init" || a == "-w" || strings.HasPrefix(a, "--watch") {
				return true
			}
		case FamilySass:
			if a == "-w" || a == "-i" || a == "--interactive" || a == "--stdin" || strings.HasPrefix(a, "--watch") {
				return true
			}
		case FamilyPostCSS:
			if a == "-w" || a == "-r" || a == "--replace" || strings.HasPrefix(a, "--watch") {
				return true
			}
		}
	}
	return false
}
